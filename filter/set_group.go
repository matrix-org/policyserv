package filter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/matrix-org/policyserv/harms"
	"github.com/matrix-org/policyserv/metrics"
)

type SetGroupConfig struct {
	// The filter names this group enables. All filters within a group are executed concurrently.
	EnabledNames []string

	// Which content classes are checked by this set group.
	RunOnClasses []harms.ContentClass
}

type setGroup struct {
	filters      []Instanced
	runOnClasses []harms.ContentClass
}

// checkEvent - If the group is meant to be run against the content class/info, processes the event through the group's
// filters. The resulting content info is the "most severe" outcome of all filters combined. Errors are collated into a
// single error. Filters are run concurrently.
func (g *setGroup) checkEvent(ctx context.Context, infoSoFar *harms.ContentInfo, input *EventInput) (*harms.ContentInfo, error) {
	return g.runFilters(infoSoFar, func(unknownFilter Instanced, ch chan setGroupRet) {
		filter, ok := unknownFilter.(InstancedEventFilter)
		if !ok {
			log.Printf("[%s | %s] Filter %T is not an InstancedEventFilter - skipping", input.Event.EventID(), input.Event.RoomID().String(), unknownFilter)
			// we force a nil response rather than an error to ensure we simply skip it
			ch <- setGroupRet{unknownFilter, nil, nil}
			return
		}

		log.Printf("[%s | %s] Running filter %T", input.Event.EventID(), input.Event.RoomID().String(), filter)
		t := metrics.StartFilterTimer(input.Event.RoomID().String(), filter.Name())
		info, err := filter.CheckEvent(ctx, input)
		t.ObserveDuration()
		input.auditContext.AppendFilterResponse(filter.Name(), info)
		g.logFilterClassifications(fmt.Sprintf("%s | %s", input.Event.EventID(), input.Event.RoomID().String()), filter, info, err)
		ch <- setGroupRet{filter, info, err}
	})
}

// checkText - The same as checkEvent, but for text content.
func (g *setGroup) checkText(ctx context.Context, infoSoFar *harms.ContentInfo, input string) (*harms.ContentInfo, error) {
	return g.runFilters(infoSoFar, func(unknownFilter Instanced, ch chan setGroupRet) {
		filter, ok := unknownFilter.(InstancedTextFilter)
		if !ok {
			log.Printf("[CheckText] Filter %T is not an InstancedTextFilter - skipping", unknownFilter)
			// we force a nil response rather than an error to ensure we simply skip it
			ch <- setGroupRet{unknownFilter, nil, nil}
			return
		}

		log.Printf("[CheckText] Running filter %T", filter)
		// TODO: Metrics
		// TODO: Audit context/webhooks
		info, err := filter.CheckText(ctx, input)
		g.logFilterClassifications("CheckText", filter, info, err)
		ch <- setGroupRet{filter, info, err}
	})
}

func (g *setGroup) logFilterClassifications(prefix string, filter Instanced, info *harms.ContentInfo, err error) {
	log.Printf("[%s] Filter %T returned %s %v", prefix, filter, info.Class(), info.Harms())
	if err != nil {
		log.Printf("[%s] Filter %T returned error: %s", prefix, filter, err)
	}
}

func (g *setGroup) runFilters(infoSoFar *harms.ContentInfo, checkFn func(f Instanced, ch chan setGroupRet)) (*harms.ContentInfo, error) {
	// First, are we within range to actually process anything?
	if !slices.Contains(g.runOnClasses, infoSoFar.Class()) {
		// No - return nothing/neutral
		return harms.NeutralContent(), nil
	}

	ch := make(chan setGroupRet, len(g.filters))
	defer close(ch)

	// Run all the filters concurrently
	for _, f := range g.filters {
		go checkFn(f, ch)
	}

	// Capture all of the results
	rets := make([]setGroupRet, len(g.filters))
	for i := 0; i < len(g.filters); i++ {
		rets[i] = <-ch
	}

	// Scan for errors and prepare a merged content info result
	contentClass := harms.ContentClassNeutral
	harmIds := make([]harms.Harm, 0)
	errs := make([]error, 0)
	for _, r := range rets {
		if r.Err != nil {
			errs = append(errs, r.Err)
			continue
		}
		harmIds = append(harmIds, r.Info.Harms()...)
		if contentClass < r.Info.Class() {
			contentClass = r.Info.Class()
		}
	}
	if len(errs) > 0 {
		// explode
		return nil, errors.Join(errs...)
	}
	return harms.NewContentInfo(contentClass, harmIds...), nil
}

type setGroupRet struct {
	Filter Instanced
	Info   *harms.ContentInfo
	Err    error
}
