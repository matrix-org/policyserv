package content

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"

	"github.com/matrix-org/policyserv/filter/classification"
)

type hashResponse map[string]string // type -> hash
type lookupResponse map[string]any  // bank name -> info object that we aren't concerned with

type HMAScanner struct {
	apiBaseUrl       string
	apiKey           string
	enabledBankNames []string
}

func NewHMAScanner(apiBaseUrl string, apiKey string, enabledBankNames []string) (*HMAScanner, error) {
	return &HMAScanner{
		apiBaseUrl:       apiBaseUrl,
		apiKey:           apiKey,
		enabledBankNames: enabledBankNames,
	}, nil
}

func (s *HMAScanner) Scan(ctx context.Context, contentType Type, content []byte) ([]classification.Classification, error) {
	hash, err := s.hash(contentType, content)
	if err != nil {
		return nil, err
	}

	matchedBankNames, err := s.match(hash)
	if err != nil {
		return nil, err
	}

	for _, matchedBankName := range matchedBankNames {
		for _, enabledBankName := range s.enabledBankNames {
			if enabledBankName == matchedBankName {
				// TODO: Support labeling the banks rather than assuming it's always CSAM
				return []classification.Classification{classification.Spam, classification.CSAM}, nil
			}
		}
	}

	return nil, nil
}

func (s *HMAScanner) hash(contentType Type, content []byte) (hashResponse, error) {
	// HMA uses a multipart form to hash content. We'll need to make that form first, then send it.

	buf := bytes.Buffer{}
	writer := multipart.NewWriter(&buf)
	var part io.Writer
	var err error
	if contentType == TypePhoto {
		part, err = writer.CreateFormFile("photo", "x")
	} else if contentType == TypeVideo {
		part, err = writer.CreateFormFile("video", "x")
	} else {
		return nil, errors.New("can only hash photos and videos")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create form file for hashing: %w", err)
	}

	_, err = part.Write(content)
	if err != nil {
		return nil, fmt.Errorf("failed to write content to form file for hashing: %w", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close multipart writer for hashing: %w", err)
	}

	// Now that we have a form, send it to HMA for hashing. We could theoretically do the hashing locally, but
	// there's a non-zero chance that the HMA deployment is more performant than we can be.
	path, err := url.JoinPath(s.apiBaseUrl, "/h/hash")
	if err != nil {
		return nil, fmt.Errorf("failed to join path for hashing: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, path, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for hashing: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request for hashing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code for hashing %d", resp.StatusCode)
	}

	// Note: hashResponse is a map[string]string, keyed by hash type. We probably only care about the
	// first key, but we return the results verbatim anyway so later code can pick it apart.
	hash := hashResponse{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&hash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response for hashing: %w", err)
	}
	return hash, nil
}

type matchResult struct {
	Err       error
	BankNames []string
}

func (s *HMAScanner) match(hash hashResponse) ([]string, error) {
	matches := make([]matchResult, len(hash))
	lock := sync.Mutex{}
	wg := sync.WaitGroup{}
	for signalType, signal := range hash {
		wg.Add(1)
		go func(signalType string, signal string) {
			defer wg.Done()
			bankNames, err := s.lookup(signalType, signal)
			log.Printf("lookup for %s:%s returned %v", signalType, signal, bankNames)
			lock.Lock()
			defer lock.Unlock()
			matches = append(matches, matchResult{
				Err:       err,
				BankNames: bankNames,
			})
		}(signalType, signal)
	}

	wg.Wait() // wait for all of the lookups to finish

	errs := make([]error, 0)
	bankNames := make([]string, 0)
	for _, match := range matches {
		if match.Err != nil {
			errs = append(errs, match.Err)
		} else {
			for _, bankName := range match.BankNames {
				bankNames = append(bankNames, bankName)
			}
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("%d errors matching: %w", len(errs), errors.Join(errs...))
	}

	return bankNames, nil
}

func (s *HMAScanner) lookup(signalType string, signal string) ([]string, error) {
	path, err := url.JoinPath(s.apiBaseUrl, "/m/lookup")
	if err != nil {
		return nil, fmt.Errorf("failed to join path for lookup: %w", err)
	}
	req, err := http.NewRequest(http.MethodGet, path+fmt.Sprintf("?signal_type=%s&signal=%s", url.QueryEscape(signalType), url.QueryEscape(signal)), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for lookup: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiKey))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request for lookup: %w", err)
	}

	lookup := lookupResponse{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&lookup)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response for lookup: %w", err)
	}

	bankNames := make([]string, 0)
	for bankName := range lookup {
		bankNames = append(bankNames, bankName)
	}

	return bankNames, nil
}
