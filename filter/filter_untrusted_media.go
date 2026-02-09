package filter

import (
	"context"
	"log"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/internal"
	"github.com/matrix-org/policyserv/trust"
)

const UntrustedMediaFilterName = "UntrustedMediaFilter"

func init() {
	mustRegister(UntrustedMediaFilterName, &UntrustedMediaFilter{})
}

type UntrustedMediaFilter struct {
}

func (m *UntrustedMediaFilter) MakeFor(set *Set) (Instanced, error) {
	communitySource, err := trust.NewSelfDirectedSource(
		set.storage,
		internal.Dereference(set.communityConfig.UntrustedMediaFilterAllowedUserGlobs),
		internal.Dereference(set.communityConfig.UntrustedMediaFilterDeniedUserGlobs),
	)
	if err != nil {
		return nil, err
	}

	trustSources := []trust.Source{
		communitySource, // always add the community's self-directed source
	}

	if internal.Dereference(set.communityConfig.UntrustedMediaFilterUseMuninn) {
		s, err := trust.NewMuninnHallSource(set.storage)
		if err != nil {
			return nil, err
		}
		trustSources = append(trustSources, s)
	}

	if internal.Dereference(set.communityConfig.UntrustedMediaFilterUsePowerLevels) {
		s, err := trust.NewCreatorSource(set.storage)
		if err != nil {
			return nil, err
		}
		trustSources = append(trustSources, s)

		s2, err := trust.NewPowerLevelsSource(set.storage)
		if err != nil {
			return nil, err
		}
		trustSources = append(trustSources, s2)
	}

	return &InstancedUntrustedMediaFilter{
		set:          set,
		trustSources: trustSources,
		upstreamFilter: &InstancedMediaFilter{
			set:        set,
			mediaTypes: internal.Dereference(set.communityConfig.UntrustedMediaFilterMediaTypes),
		},
	}, nil
}

type InstancedUntrustedMediaFilter struct {
	set            *Set
	trustSources   []trust.Source
	upstreamFilter *InstancedMediaFilter
}

func (f *InstancedUntrustedMediaFilter) Name() string {
	return UntrustedMediaFilterName
}

func (f *InstancedUntrustedMediaFilter) CheckEvent(ctx context.Context, input *EventInput) ([]classification.Classification, error) {
	if !input.Event.SenderID().IsUserID() {
		return nil, nil // unable to form an opinion on this event
	}

	userId := input.Event.SenderID().ToUserID().String()
	isTrusted := false
	for _, source := range f.trustSources {
		has, err := source.HasCapability(ctx, userId, input.Event.RoomID().String(), trust.CapabilityMedia)
		if err != nil {
			return nil, err
		}
		if has == trust.TristateTrue {
			log.Printf("[%s | %s] %T provides %s with media capability", input.Event.EventID(), input.Event.RoomID(), source, userId)
			isTrusted = true
			// there may still be a deny in the array, so we continue
		} else if has == trust.TristateFalse {
			log.Printf("[%s | %s] %T denies %s the media capability", input.Event.EventID(), input.Event.RoomID(), source, userId)
			isTrusted = false
			break // deny wins, so break
		} else {
			// log default responses, but don't do anything with them (our filter's default is to deny)
			log.Printf("[%s | %s] %T has no opinion on media capability for %s", input.Event.EventID(), input.Event.RoomID(), source, userId)
		}
	}

	if isTrusted {
		return nil, nil // no useful opinion on this event - the sender is trusted to send whatever we're about to check for
	}

	// copy logic from the regular media filter
	return f.upstreamFilter.CheckEvent(ctx, input)
}
