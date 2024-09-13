package state

import (
	"context"
	"fmt"
	gatewayv1beta1 "github.com/kyma-project/api-gateway/apis/gateway/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrl "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
	"time"
)

// FinalizerHandler represents an example of a function which implements
// a HandlerFunc. That function ensures that the finalizer is set up in
// a resource, and if it is not set up, e.g. the resource has just been created,
// updates the resource and stops the execution of a reconciliation, after the
// result of update got returned.
// This function works on kubernetes objects, so this function can be used in
// many resources that need to have a finalizer.
func FinalizerHandler(obj client.Object, key string) HandlerFunc {
	return func(ctx context.Context, s *State) error {
		if !ctrl.ContainsFinalizer(obj, key) {
			s.Log().Info("deletion finalizer not found, adding")
			// stopping execution
			defer s.Stop()
			ctrl.AddFinalizer(obj, key)
			return s.Client().Update(ctx, obj)
		}
		return nil
	}
}

// DeletionHandler is an example of implementing the deletion logic implemented
// as HandlerFunc. This creates a function that checks if K8s object contains
// a deletion timestamp, and if so, removes a finalizer and finishes execution
// of the workflow.
//
// This implementation works on kubernetes objects, so can be used for any type
// of resources that implement client.Object interface.
func DeletionHandler(obj client.Object, final string) HandlerFunc {
	return func(ctx context.Context, s *State) error {
		if obj.GetDeletionTimestamp() == nil {
			return nil
		}
		s.Log().Info("resource is in deletion. Removing finalizer")
		defer s.Stop()
		ctrl.RemoveFinalizer(obj, final)
		return s.Client().Delete(ctx, obj)
	}
}

// This simple test scenario showcases how State is handled during the operations
// on an underlying object. The resource is tested on static set of handlers
// that modify or edit existing object taken from the API Server on the example
// of changing and modifying a finalizer.
//
// The rule that is containing a finalizer does not break the handler loop, and
// all subsequent handlers are safely executed.
func TestState_Run(t *testing.T) {
	testcases := []struct {
		name          string
		rule          gatewayv1beta1.APIRule
		wantFinalizer bool
	}{
		{
			name: "rule does not contain finalizer, no error",
			rule: gatewayv1beta1.APIRule{ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test"},
			},
			wantFinalizer: true,
		},
		{
			name: "rule contains finalizer, no error",
			rule: gatewayv1beta1.APIRule{ObjectMeta: metav1.ObjectMeta{
				Name:       "test",
				Namespace:  "test",
				Finalizers: []string{"finalizer"}},
			},
			wantFinalizer: true,
		},
		{
			name: "rule contains deletion timestamp, remove resource",
			rule: gatewayv1beta1.APIRule{ObjectMeta: metav1.ObjectMeta{
				Name:              "test",
				Namespace:         "test",
				Finalizers:        []string{"finalizer"},
				DeletionTimestamp: &metav1.Time{Time: time.Now()}},
			},
			wantFinalizer: false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			sch := runtime.NewScheme()
			if err := gatewayv1beta1.AddToScheme(sch); err != nil {
				t.Fatal(err)
			}
			c := fake.NewClientBuilder().
				WithScheme(sch).
				WithObjects(&tc.rule).WithStatusSubresource(&tc.rule).Build()

			s := New(c, WithLogger(zap.New()))
			exampleFunc := func(text string) HandlerFunc {
				return func(ctx context.Context, s *State) error {
					fmt.Println(text)
					return nil
				}
			}
			s.AddHandlers(
				FinalizerHandler(&tc.rule, "finalizer"),
				DeletionHandler(&tc.rule, "finalizer"),
				exampleFunc("Mama, just killed a man."),
				exampleFunc("Put a gun against his head."),
				exampleFunc("Pulled my trigger, now he's dead."),
			)

			err := s.Run(context.TODO())
			if err != nil {
				t.Fatal(err)
			}

			if !ctrl.ContainsFinalizer(&tc.rule, "finalizer") && tc.wantFinalizer {
				t.Error("finalizer not added, but should have been")
			}
		})
	}
}
