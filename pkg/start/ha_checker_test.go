package start

import (
	"context"
	"testing"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func TestWaitForAvailabilityBeforeTearDown(t *testing.T) {
	tests := []struct {
		name     string
		pollers  []*poller
		errCount int
	}{
		{
			name: "all conditions satisfy",
			pollers: []*poller{
				{
					timeout:   time.Hour,
					what:      "foo",
					condition: func(context.Context) (string, bool) { return "satisfied", true },
				},
				{
					timeout:   time.Hour,
					what:      "bar",
					condition: func(context.Context) (string, bool) { return "satisfied", true },
				},
			},
			errCount: 0,
		},
		{
			name: "all conditions timeout",
			pollers: []*poller{
				{
					timeout:   5 * time.Second,
					what:      "foo",
					condition: func(context.Context) (string, bool) { return "not satisfied yet", false },
				},
				{
					timeout:   5 * time.Second,
					what:      "bar",
					condition: func(context.Context) (string, bool) { return "not satisfied yet", false },
				},
			},
			errCount: 2,
		},
		{
			name: "some conditions timeout",
			pollers: []*poller{
				{
					timeout:   5 * time.Second,
					what:      "foo",
					condition: func(context.Context) (string, bool) { return "satisfied", true },
				},
				{
					timeout:   5 * time.Second,
					what:      "bar",
					condition: func(context.Context) (string, bool) { return "not satisfied yet", false },
				},
			},
			errCount: 1,
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := waitFor(test.pollers)
			switch {
			case test.errCount > 0:
				if err == nil {
					t.Fatalf("expected error")
				}
				aggregateErr, ok := err.(utilerrors.Aggregate)
				if !ok {
					t.Fatalf("expected aggregated error, but got: %T", err)
				}
				if want, got := test.errCount, len(aggregateErr.Errors()); want != got {
					t.Errorf("expected %d errors, but got: %d", want, got)
				}
			default:
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
