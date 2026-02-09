package filter

import (
	"context"
	"errors"
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
	// First, are we within range to actually process anything?
	spamVec := input.IncrementalConfidenceVectors.GetVector(classification.Spam)
	if spamVec < g.minSpamVectorValue || spamVec > g.maxSpamVectorValue { // don't use equality here because it'll exclude "max: 1.0" configs
		// No - return nothing
		return confidence.NewConfidenceVectors(), nil
	}

	// This is just a small container type to make passing values over channels easier
	type ret struct {
		Instanced
		Classifications []classification.Classification
		error
	}

	ch := make(chan ret, len(g.filters))
	defer close(ch)

	// Run all the filters concurrently
	for _, f := range g.filters {
		go func(unknownFilter Instanced, ch chan ret, input *EventInput) {
			filter, ok := unknownFilter.(InstancedEventFilter)
			if !ok {
				log.Printf("[%s | %s] Filter %T is not an InstancedEventFilter - skipping", input.Event.EventID(), input.Event.RoomID().String(), unknownFilter)
				// we force a nil response rather than an error to ensure we simply skip it
				ch <- ret{unknownFilter, nil, nil}
				return
			}

			log.Printf("[%s | %s] Running filter %T", input.Event.EventID(), input.Event.RoomID().String(), filter)
			t := metrics.StartFilterTimer(input.Event.RoomID().String(), filter.Name())
			classifications, err := filter.CheckEvent(ctx, input)
			t.ObserveDuration()
			strClassifications := make([]string, 0, len(classifications))
			for _, cls := range classifications {
				// We don't use .String() here because that will uninvert values, making logs harder to follow.
				strClassifications = append(strClassifications, string(cls))
			}
			input.auditContext.AppendFilterResponse(filter.Name(), classifications)
			log.Printf("[%s | %s] Filter %T returned %v", input.Event.EventID(), input.Event.RoomID().String(), filter, strClassifications)
			if err != nil {
				log.Printf("[%s | %s] Filter %T returned error: %s", input.Event.EventID(), input.Event.RoomID().String(), filter, err)
				// don't return early - we want to pass all the things through at the same time
			}
			ch <- ret{filter, classifications, err}
		}(f, ch, input)
	}

	// Capture all of the results
	rets := make([]ret, len(g.filters))
	for i := 0; i < len(g.filters); i++ {
		rets[i] = <-ch
	}

	// Scan for errors and prepare for a final confidence vectors result
	vecs := confidence.NewConfidenceVectors()
	errs := make([]error, 0)
	for _, r := range rets {
		if r.error != nil {
			errs = append(errs, r.error)
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
