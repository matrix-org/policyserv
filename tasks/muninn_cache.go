package tasks

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/storage"
	"github.com/matrix-org/policyserv/trust"
)

func CacheMuninnHallMembers(db storage.PersistentStorage, cnf *config.InstanceConfig) {
	if cnf.MuninnHallSourceApiKey == "" || cnf.MuninnHallSourceApiUrl == "" {
		log.Println("Skipping fetching Muninn Hall members: no API key or URL configured")
		return // nothing to do
	}

	log.Println("Fetching Muninn Hall members...")

	req, err := http.NewRequest(http.MethodGet, cnf.MuninnHallSourceApiUrl, nil)
	if err != nil {
		log.Printf("Failed to create request for Muninn Hall source: %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+cnf.MuninnHallSourceApiKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch Muninn Hall source: %v", err)
		return
	}
	defer res.Body.Close()

	dir := make(trust.MuninnHallMemberDirectory)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&dir)
	if err != nil {
		log.Printf("Failed to decode Muninn Hall source: %v", err)
		return
	}

	source, err := trust.NewMuninnHallSource(db)
	if err != nil {
		log.Printf("Failed to create Muninn Hall source: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute) // 1 minute should be plenty time to persist
	defer cancel()
	err = source.ImportData(ctx, dir)
	if err != nil {
		log.Printf("Failed to import into Muninn Hall source: %v", err)
		return
	}

	log.Println("Finished fetching Muninn Hall members")
}
