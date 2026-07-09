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
	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/media"
	"github.com/matrix-org/policyserv/notifiers"
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
	notifier        notifiers.MatrixNotifier
	contentScanner  content.Scanner // may be nil if scanning not possible
	groups          []*setGroup
	communityConfig *config.CommunityConfig
	instanceConfig  *config.InstanceConfig
	communityId     string
}

func NewSet(config *SetConfig, storage storage.PersistentStorage, pubsub pubsub.Client, notifier notifiers.MatrixNotifier, contentScanner content.Scanner) (*Set, error) {
	if config.CommunityId == "" {
		config.CommunityId = "default"
	}
	set := &Set{
		storage:         storage,
		pubsub:          pubsub,
		notifier:        notifier,
		contentScanner:  contentScanner,
		groups:          make([]*setGroup, len(config.Groups)),
		communityConfig: config.CommunityConfig,
		instanceConfig:  config.InstanceConfig,
		communityId:     config.CommunityId,
	}
	for i, groupCnf := range config.Groups {
		set.groups[i] = &setGroup{
			filters:               make([]Instanced, 0),
			checkedContentClasses: groupCnf.CheckedContentClasses,
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

// CheckEvent - Checks an event over all of the set groups in order. If a set group errors, execution stops there.
// Note: the mediaDownloader may be nil to prevent parsing and downloading of media. This should only be done in test environments.
func (s *Set) CheckEvent(ctx context.Context, event gomatrixserverlib.PDU, mediaDownloader media.Downloader) (*harms.ContentInfo, error) {
	log.Printf("[%s | %s | %s] Checking event", event.EventID(), event.RoomID().String(), s.communityId)

	if !event.SenderID().IsUserID() || event.SenderID().ToUserID() == nil {
		log.Printf("[%s | %s] Skipping event and flagging as spam because sender is not a user", event.EventID(), event.RoomID().String())
		return harms.ProhibitedContent(harms.SpamGeneral, harms.PolicyservSpecNonCompliance), nil
	}

	contentClass := harms.ContentClassNeutral
	harmIds := make([]harms.Harm, 0)
	auditCtx, err := newAuditContext(s.notifier, s.communityId, event)
	if err != nil {
		return nil, err
	}
	for i, group := range s.groups {
		input := &EventInput{
			Event:        event,
			auditContext: auditCtx,
			Medias:       make([]*media.Item, 0),
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

		info, err := group.checkEvent(ctx, harms.NewContentInfo(contentClass, harmIds...), input)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("error at group %d", i), err)
		}
		if info.Class() > contentClass {
			contentClass = info.Class()
		}
		harmIds = append(harmIds, info.Harms()...)
	}

	info := harms.NewContentInfo(contentClass, harmIds...)
	auditCtx.IsSpam = info.Class() == harms.ContentClassProhibited
	go func(auditCtx *auditContext, s *Set) { // run the audit publishing async to avoid blocking the hot path any more than required
		err := auditCtx.Publish()
		if err != nil {
			log.Printf("[%s | %s] Non-fatal error publishing audit: %s", auditCtx.Event.EventID(), auditCtx.Event.RoomID().String(), err)
		}
	}(auditCtx, s)
	return info, nil
}

func (s *Set) CheckText(ctx context.Context, text string) (*harms.ContentInfo, error) {
	log.Printf("[CheckText | %s] Checking text", s.communityId)
	contentClass := harms.ContentClassNeutral
	harmIds := make([]harms.Harm, 0)
	// TODO: Audit context/webhooks
	for i, group := range s.groups {
		info, err := group.checkText(ctx, harms.NewContentInfo(contentClass, harmIds...), text)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("error at group %d", i), err)
		}
		if info.Class() > contentClass {
			contentClass = info.Class()
		}
		harmIds = append(harmIds, info.Harms()...)
	}
	return harms.NewContentInfo(contentClass, harmIds...), nil
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
