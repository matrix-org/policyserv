package filter

import (
	"context"
	"encoding/json"
	"strings"

	goSet "github.com/deckarep/golang-set"
	"github.com/matrix-org/policyserv/filter/classification"
)

const MentionsFilterName = "MentionsFilter"

func init() {
	mustRegister(MentionsFilterName, &MentionsFilter{})
}

type MentionsFilter struct {
}

func (m *MentionsFilter) MakeFor(set *Set) (Instanced, error) {
	return &InstancedMentionsFilter{
		set:           set,
		maxMentions:   set.communityConfig.MentionFilterMaxMentions,
		minNameLength: set.communityConfig.MentionFilterMinPlaintextLength,
	}, nil
}

type InstancedMentionsFilter struct {
	set           *Set
	maxMentions   int
	minNameLength int
}

func (f *InstancedMentionsFilter) Name() string {
	return MentionsFilterName
}

func (f *InstancedMentionsFilter) CheckEvent(ctx context.Context, input *Input) ([]classification.Classification, error) {
	roomId := input.Event.RoomID().String()

	// Return early on non-message events
	if input.Event.Type() != "m.room.message" {
		return nil, nil
	}

	content := &mentionsContent{}
	err := json.Unmarshal(input.Event.Content(), &content)
	if err != nil {
		return nil, err
	}

	rawUserIds, rawDisplayNames, err := f.set.storage.GetUserIdsAndDisplayNamesByRoomId(ctx, roomId)
	if err != nil {
		return nil, err
	}

	displayNames := goSet.NewSet()
	for _, displayName := range rawDisplayNames {
		if len(displayName) == 0 || len(displayName) < f.minNameLength {
			continue
		}
		displayNames.Add(displayName)
	}
	userIds := goSet.NewSet()
	for _, userId := range rawUserIds {
		if len(userId) == 0 {
			continue
		}
		userIds.Add(userId)
	}
	mentionedUserIds := goSet.NewSet()
	for _, userId := range content.Mentions.UserIDs {
		mentionedUserIds.Add(userId)
	}

	// get the number of user IDs mentioned
	numMentionedUserIds := userIds.Intersect(mentionedUserIds).Cardinality()
	if numMentionedUserIds >= f.maxMentions {
		return []classification.Classification{
			classification.Spam,
			classification.Mentions,
		}, nil
	}

	// now we check the body for either user ID or display name matches.
	// Attackers may be funny and set their display name to 'the' to try to trip this up,
	// so we care about the variety in the display names, which is why we use a set of display
	// names and not the total list of display names.
	loopIter := func(i interface{}) bool {
		userIdOrDisplayName := i.(string)
		if strings.Contains(content.Body, userIdOrDisplayName) {
			numMentionedUserIds++
		} else if strings.Contains(content.FormattedBody, userIdOrDisplayName) {
			numMentionedUserIds++
		}
		return numMentionedUserIds >= f.maxMentions
	}
	userIds.Each(loopIter)
	if numMentionedUserIds >= f.maxMentions {
		return []classification.Classification{
			classification.Spam,
			classification.Mentions,
		}, nil
	}
	displayNames.Each(loopIter)
	if numMentionedUserIds >= f.maxMentions {
		return []classification.Classification{
			classification.Spam,
			classification.Mentions,
		}, nil
	}

	return nil, nil
}

type mentionsContent struct {
	Mentions struct {
		UserIDs []string `json:"user_ids"`
	} `json:"m.mentions"`
	Body          string `json:"body"`
	FormattedBody string `json:"formatted_body"`
}
