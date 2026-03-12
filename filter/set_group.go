package filter

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/matrix-org/policyserv/filter/classification"
	"github.com/matrix-org/policyserv/filter/confidence"
	"github.com/matrix-org/policyserv/metrics"
)

type SetGroupConfig struct {
	// The filter names this group enables. All filters within a group are executed concurrently.
	EnabledNames []string

	// The minimum vector value for classification.Spam required for this set group to process events.
	// Note: The first set group will receive a classification.Spam vector value of 0.5
	MinimumSpamVectorValue float64

	// The maximum vector value for classification.Spam allowed for this set group to process events.
	MaximumSpamVectorValue float64
}

type setGroup struct {
	filters            []Instanced
	minSpamVectorValue float64
	maxSpamVectorValue float64
}

// checkEvent - If the input's spam vector is within range, processes the event through the group's filters and returns
// the confidence vectors of all filters combined. If the input's spam vector is out of range, returns empty confidence
// vectors. Errors are collated into a single error. Filters are run concurrently.
func (g *setGroup) checkEvent(ctx context.Context, input *EventInput) (confidence.Vectors, error) {
	spamVec := input.IncrementalConfidenceVectors.GetVector(classification.Spam)
	return g.runFilters(spamVec, func(unknownFilter Instanced, ch chan setGroupRet) {
		filter, ok := unknownFilter.(InstancedEventFilter)
		if !ok {
			log.Printf("[%s | %s] Filter %T is not an InstancedEventFilter - skipping", input.Event.EventID(), input.Event.RoomID().String(), unknownFilter)
			// we force a nil response rather than an error to ensure we simply skip it
			ch <- setGroupRet{unknownFilter, nil, nil}
			return
		}

		log.Printf("[%s | %s] Running filter %T", input.Event.EventID(), input.Event.RoomID().String(), filter)
		t := metrics.StartFilterTimer(input.Event.RoomID().String(), filter.Name())
		classifications, err := filter.CheckEvent(ctx, input)
		t.ObserveDuration()
		input.auditContext.AppendFilterResponse(filter.Name(), classifications)
		g.logFilterClassifications(fmt.Sprintf("%s | %s", input.Event.EventID(), input.Event.RoomID().String()), filter, classifications, err)
		ch <- setGroupRet{filter, classifications, err}
	})
}

// checkText - The same as checkEvent, but for text content.
func (g *setGroup) checkText(ctx context.Context, incrementalVectors confidence.Vectors, input string) (confidence.Vectors, error) {
	spamVec := incrementalVectors.GetVector(classification.Spam)
	return g.runFilters(spamVec, func(unknownFilter Instanced, ch chan setGroupRet) {
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
		classifications, err := filter.CheckText(ctx, input)
		g.logFilterClassifications("CheckText", filter, classifications, err)
		ch <- setGroupRet{filter, classifications, err}
	})
}

func (g *setGroup) logFilterClassifications(prefix string, filter Instanced, classifications []classification.Classification, err error) {
	strClassifications := make([]string, 0, len(classifications))
	for _, cls := range classifications {
		// We don't use .String() here because that will uninvert values, making logs harder to follow.
		strClassifications = append(strClassifications, string(cls))
	}
	log.Printf("[%s] Filter %T returned %v", prefix, filter, strClassifications)
	if err != nil {
		log.Printf("[%s] Filter %T returned error: %s", prefix, filter, err)
	}
}

func (g *setGroup) runFilters(spamVec float64, checkFn func(f Instanced, ch chan setGroupRet)) (confidence.Vectors, error) {
	// First, are we within range to actually process anything?
	if spamVec < g.minSpamVectorValue || spamVec > g.maxSpamVectorValue { // don't use equality here because it'll exclude "max: 1.0" configs
		// No - return nothing
		return confidence.NewConfidenceVectors(), nil
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

	// Scan for errors and prepare for a final confidence vectors result
	vecs := confidence.NewConfidenceVectors()
	errs := make([]error, 0)
	for _, r := range rets {
		if r.Err != nil {
			errs = append(errs, r.Err)
			continue
		}
		for _, cls := range r.Classifications {
			// If we've already flagged an event as spam, don't allow that to be un-flagged.
			// Note: we compare using .String() because it returns the uninverted value, if inverted.
			if cls.String() == classification.Spam.String() && vecs.GetVector(classification.Spam) > 0.5 {
				continue
			}
			vecs.SetVector(cls, 1.0)
		}
	}
	if len(errs) > 0 {
		// explode
		return nil, errors.Join(errs...)
	}
	return vecs, nil
}

type setGroupRet struct {
	Filter          Instanced
	Classifications []classification.Classification
	Err             error
}
