package filter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/matrix-org/gomatrixserverlib"
	"github.com/matrix-org/policyserv/config"
	"github.com/matrix-org/policyserv/content"
	"github.com/matrix-org/policyserv/filter/audit"
	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/media"
	"github.com/matrix-org/policyserv/pubsub"
	"github.com/matrix-org/policyserv/storage"
)

type SetConfig struct {
	// Filters are organized into Groups for execution and processing.
	Groups          []*SetGroupConfig
	CommunityConfig *config.CommunityConfig
	InstanceConfig  *config.InstanceConfig
	CommunityId     string
}

type Set struct {
	storage         storage.PersistentStorage
	pubsub          pubsub.Client
	queue           *audit.Queue
	contentScanner  content.Scanner // may be nil if scanning not possible
	groups          []*setGroup
	communityConfig *config.CommunityConfig
	instanceConfig  *config.InstanceConfig
	communityId     string
}

func NewSet(config *SetConfig, storage storage.PersistentStorage, pubsub pubsub.Client, queue *audit.Queue, contentScanner content.Scanner) (*Set, error) {
	set := &Set{
		storage:         storage,
		pubsub:          pubsub,
		queue:           queue,
		contentScanner:  contentScanner,
		groups:          make([]*setGroup, len(config.Groups)),
		communityConfig: config.CommunityConfig,
		instanceConfig:  config.InstanceConfig,
		communityId:     config.CommunityId,
	}
	for i, groupCnf := range config.Groups {
		set.groups[i] = &setGroup{
			filters:            make([]Instanced, 0),
			minSpamVectorValue: groupCnf.MinimumSpamVectorValue,
			maxSpamVectorValue: groupCnf.MaximumSpamVectorValue,
		}
		for _, name := range groupCnf.EnabledNames {
			f, err := findByName(name)
			if err != nil {
				return nil, errors.Join(fmt.Errorf("error finding filter name: %s", name))
			}
			instanced, err := f.MakeFor(set)
			if err != nil {
				return nil, errors.Join(fmt.Errorf("error making filter for: %s", name), err)
			}
			set.groups[i].filters = append(set.groups[i].filters, instanced)
		}
	}
	return set, nil
}

// GetStorage - Accessor to the underlying PersistentStorage
func (s *Set) GetStorage() storage.PersistentStorage {
	return s.storage
}

// CheckEvent - Checks an event over all of the set groups in order. If a set group errors, execution stops there.
// Note: the mediaDownloader may be nil to prevent parsing and downloading of media. This should only be done in test environments.
func (s *Set) CheckEvent(ctx context.Context, event gomatrixserverlib.PDU, mediaDownloader media.Downloader) (confidence.Vectors, error) {
	log.Printf("[%s | %s | %s] Checking event", event.EventID(), event.RoomID().String(), s.communityId)

	if !event.SenderID().IsUserID() || event.SenderID().ToUserID() == nil {
		log.Printf("[%s | %s] Skipping event and flagging as spam because sender is not a user", event.EventID(), event.RoomID().String())
		return confidence.Vectors{
			classification.Spam:          1,
			classification.NonCompliance: 1,
		}, nil
	}

	vecs := confidence.NewConfidenceVectors()
	vecs.SetVector(classification.Spam, 0.5) // per docs elsewhere, start by assuming 50% likelihood of spam
	auditCtx, err := newAuditContext(s.instanceConfig, event, internal.Dereference(s.communityConfig.WebhookUrl))
	if err != nil {
		return nil, err
	}
	for i, group := range s.groups {
		input := &EventInput{
			Event:                        event,
			IncrementalConfidenceVectors: vecs,
			auditContext:                 auditCtx,
			Medias:                       make([]*media.Item, 0),
		}

		if mediaDownloader != nil {
			// Extract media items from event, if possible.
			// TODO: In future, we'll also want to capture custom emoji and similar
			eventContent := &mediaUrlsOnly{}
			err = json.Unmarshal(input.Event.Content(), &eventContent)
			if err != nil {
				// Probably not a string
				return nil, err
			}
			tryAppendMedia := func(url string) {
				if len(url) == 0 {
					return // the field we're trying to parse probably wasn't present on the event
				}
				m, err := media.NewItem(url, mediaDownloader)
				if err != nil {
					log.Printf("[%s | %s] Non-fatal error creating new media object for '%s': %s", event.EventID(), event.RoomID().String(), url, err)
				}
				log.Printf("[%s | %s] Discovered media on event: %s", event.EventID(), event.RoomID().String(), m)
				input.Medias = append(input.Medias, m)
			}
			tryAppendMedia(eventContent.Url)
			tryAppendMedia(eventContent.Info.ThumbnailUrl)
		} else {
			log.Printf("[%s | %s] Skipping media extraction as mediaDownloader is nil", event.EventID(), event.RoomID().String())
		}

		v, err := group.checkEvent(ctx, input)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("error at group %d", i), err)
		}
		vecs = s.combineVectors(vecs, v)

		auditCtx.AppendSetGroupVectors(v)
	}
	auditCtx.FinalVectors = vecs
	auditCtx.IsSpam = s.IsSpamResponse(ctx, vecs)
	go func(auditCtx *auditContext, s *Set) { // run the audit publishing async to avoid blocking the hot path any more than required
		err := auditCtx.Publish(s.queue)
		if err != nil {
			log.Printf("[%s | %s] Non-fatal error publishing audit: %s", auditCtx.Event.EventID(), auditCtx.Event.RoomID().String(), err)
		}
	}(auditCtx, s)
	return vecs, nil
}

func (s *Set) CheckText(ctx context.Context, text string) (confidence.Vectors, error) {
	log.Printf("[CheckText | %s] Checking text", s.communityId)
	vecs := confidence.NewConfidenceVectors()
	vecs.SetVector(classification.Spam, 0.5) // per docs elsewhere, start by assuming 50% likelihood of spam
	// TODO: Audit context/webhooks
	for i, group := range s.groups {
		v, err := group.checkText(ctx, vecs, text)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("error at group %d", i), err)
		}
		vecs = s.combineVectors(vecs, v)
	}
	return vecs, nil
}

func (s *Set) combineVectors(incremental confidence.Vectors, group confidence.Vectors) confidence.Vectors {
	for cls, val := range group {
		// If we've already flagged some content as spam, don't allow that to be un-flagged.
		// Note: we compare using .String() because it returns the uninverted value, if inverted.
		if cls.String() == classification.Spam.String() && incremental.GetVector(classification.Spam) > 0.5 { // 0.5 to escape the seed value
			continue
		}
		incremental.SetVector(cls, val) // overwrite rather than average
	}
	return incremental
}

func (s *Set) IsSpamResponse(ctx context.Context, vecs confidence.Vectors) bool {
	val := vecs.GetVector(classification.Spam)
	return val >= internal.Dereference(s.communityConfig.SpamThreshold)
}

func (s *Set) Close() error {
	allErrors := make([]error, 0)
	for _, group := range s.groups {
		for _, f := range group.filters {
			if closable, ok := f.(io.Closer); ok {
				err := closable.Close()
				if err != nil {
					allErrors = append(allErrors, err)
				}
			}
		}
	}
	if len(allErrors) > 0 {
		return errors.Join(allErrors...)
	}
	return nil
}

type mediaUrlsOnly struct {
	Url  string `json:"url"`
	Info struct {
		ThumbnailUrl string `json:"thumbnail_url"`
	}
}
