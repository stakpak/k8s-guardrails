package main

import (
	"encoding/json"
	"reflect"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	v1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var getOwnerReferenceLabelsAdminStub = func(ownerReference []metav1.OwnerReference, namespace string) (map[string]string, error) {
	return map[string]string{labelKey: "admin"}, nil
}
var getOwnerReferenceLabelsEmptyStub = func(ownerReference []metav1.OwnerReference, namespace string) (map[string]string, error) {
	return map[string]string{}, nil
}

func TestOnMutate(t *testing.T) {
	testCases := []struct {
		name                        string
		objectMeta                  metav1.ObjectMeta
		operation                   string
		username                    string
		expectedLabels              map[string]string
		allowed                     bool
		getOwnerReferenceLabelsStub func(ownerReference []metav1.OwnerReference, namespace string) (map[string]string, error)
	}{
		{
			name: "create add label",
			objectMeta: metav1.ObjectMeta{
				Labels: nil,
			},
			operation:      "CREATE",
			username:       "admin",
			expectedLabels: map[string]string{labelKey: "admin"},
			allowed:        true,
		},
		{
			name: "create don't add label",
			objectMeta: metav1.ObjectMeta{
				Labels: nil,
			},
			operation:      "CREATE",
			username:       "joe",
			expectedLabels: nil,
			allowed:        true,
		},
		{
			name: "create append label",
			objectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"another-label": "true"},
			},
			operation:      "CREATE",
			username:       "admin",
			expectedLabels: map[string]string{"another-label": "true", labelKey: "admin"},
			allowed:        true,
		},
		{
			name: "update allow",
			objectMeta: metav1.ObjectMeta{
				Labels: map[string]string{labelKey: "admin"},
			},
			operation:      "UPDATE",
			username:       "admin",
			expectedLabels: map[string]string{labelKey: "admin"},
			allowed:        true,
		},
		{
			name: "update disallow",
			objectMeta: metav1.ObjectMeta{
				Labels: map[string]string{labelKey: "joe"},
			},
			operation:      "UPDATE",
			username:       "admin",
			expectedLabels: map[string]string{labelKey: "joe"},
			allowed:        false,
		},
		{
			name: "update disallow no label",
			objectMeta: metav1.ObjectMeta{
				Labels: nil,
			},
			operation:      "UPDATE",
			username:       "admin",
			expectedLabels: nil,
			allowed:        false,
		},
		{
			name: "delete allow",
			objectMeta: metav1.ObjectMeta{
				Labels: map[string]string{labelKey: "admin"},
			},
			operation:      "DELETE",
			username:       "admin",
			expectedLabels: map[string]string{labelKey: "admin"},
			allowed:        true,
		},
		{
			name: "delete disallow",
			objectMeta: metav1.ObjectMeta{
				Labels: map[string]string{labelKey: "joe"},
			},
			operation:      "DELETE",
			username:       "admin",
			expectedLabels: map[string]string{labelKey: "joe"},
			allowed:        false,
		},
		{
			name: "delete disallow no label",
			objectMeta: metav1.ObjectMeta{
				Labels: nil,
			},
			operation:      "DELETE",
			username:       "admin",
			expectedLabels: nil,
			allowed:        false,
		},
		{
			name: "update out of scope allow",
			objectMeta: metav1.ObjectMeta{
				Labels: map[string]string{labelKey: "admin"},
			},
			operation:      "UPDATE",
			username:       "joe",
			expectedLabels: map[string]string{labelKey: "admin"},
			allowed:        true,
		},
		{
			name: "update out of scope allow no label",
			objectMeta: metav1.ObjectMeta{
				Labels: nil,
			},
			operation:      "UPDATE",
			username:       "joe",
			expectedLabels: nil,
			allowed:        true,
		},
		{
			name: "create add label from owner ref",
			objectMeta: metav1.ObjectMeta{
				Labels: nil,
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: "any",
					},
				},
			},
			operation:                   "CREATE",
			username:                    "system",
			expectedLabels:              map[string]string{labelKey: "admin"},
			allowed:                     true,
			getOwnerReferenceLabelsStub: getOwnerReferenceLabelsAdminStub,
		},
		{
			name: "create add label from service account ref",
			objectMeta: metav1.ObjectMeta{
				Labels: nil,
				Annotations: map[string]string{
					"kubernetes.io/service-account.name": "sa",
				},
			},
			operation:                   "CREATE",
			username:                    "system",
			expectedLabels:              map[string]string{labelKey: "admin"},
			allowed:                     true,
			getOwnerReferenceLabelsStub: getOwnerReferenceLabelsAdminStub,
		},
		{
			name: "create no owner label found",
			objectMeta: metav1.ObjectMeta{
				Labels: nil,
			},
			operation:                   "CREATE",
			username:                    "system",
			expectedLabels:              nil,
			allowed:                     true,
			getOwnerReferenceLabelsStub: getOwnerReferenceLabelsEmptyStub,
		},
	}

	owners = []string{"admin"}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			request := corev1.Secret{ObjectMeta: tc.objectMeta}
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			review := v1.AdmissionReview{
				Request: &v1.AdmissionRequest{
					Kind: metav1.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "Secret",
					},
					Operation: v1.Operation(tc.operation),
					UserInfo:  authenticationv1.UserInfo{Username: tc.username},
				},
			}
			if tc.operation == "DELETE" {
				review.Request.OldObject = runtime.RawExtension{Raw: raw}
			} else {
				review.Request.Object = runtime.RawExtension{Raw: raw}
			}
			if tc.getOwnerReferenceLabelsStub != nil {
				getOwnerReferenceLabels = tc.getOwnerReferenceLabelsStub
			}
			response := onMutate(review)
			if response.Allowed != tc.allowed {
				t.Errorf("\nexpected allowed to be %#v, got %#v", tc.allowed, response.Allowed)
			}
			if response.Patch != nil {
				patchObj, err := jsonpatch.DecodePatch([]byte(response.Patch))
				if err != nil {
					t.Fatal(err)
				}
				raw, err = patchObj.Apply(raw)
				if err != nil {
					t.Fatal(err)
				}
			}

			objType := reflect.TypeOf(request)
			objTest := reflect.New(objType).Interface()
			err = json.Unmarshal(raw, objTest)
			if err != nil {
				t.Fatal(err)
			}
			actual := objTest.(*corev1.Secret)
			if !reflect.DeepEqual(actual.Labels, tc.expectedLabels) {
				t.Errorf("\nexpected %#v, got %#v, patch: %v", tc.expectedLabels, actual.Labels, string(response.Patch))
			}
		})
	}
}
