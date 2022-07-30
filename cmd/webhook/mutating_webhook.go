/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func getFirstLabelPatch(labelValue string) string {
	return fmt.Sprintf(`[
		{ "op": "add", "path": "/metadata/labels", "value": {"%s": "%s"}}
	]`, labelKey, labelValue)
}

func getAdditionalLabelPatch(labelValue string) string {
	return fmt.Sprintf(`[
		{ "op": "add", "path": "/metadata/labels/%s", "value": "%s" }
	]`, strings.ReplaceAll(labelKey, "/", "~1"), labelValue)
}

// func getUpdateLabelPatch(labelValue string) string {
// 	return fmt.Sprintf(`[
// 		{ "op": "replace", "path": "/metadata/labels/%s", "value": "%s" }
// 	]`, strings.ReplaceAll(labelKey, "/", "~1"), labelValue)
// }
func ServeOnMutate(w http.ResponseWriter, r *http.Request) {
	serve(w, r, newDelegateToV1AdmitHandler(onMutate))
}

func onMutate(ar v1.AdmissionReview) *v1.AdmissionResponse {
	klog.V(2).Info("calling on mutate")

	obj := struct {
		metav1.ObjectMeta `json:"metadata,omitempty"`
	}{}

	var raw []byte

	if ar.Request.Operation == v1.Delete {
		raw = ar.Request.OldObject.Raw
	} else {
		raw = ar.Request.Object.Raw
	}

	klog.V(2).Info("getting metadata")
	err := json.Unmarshal(raw, &obj)
	if err != nil {
		klog.Error(err)
		return toV1AdmissionResponse(err)
	}

	reviewResponse := v1.AdmissionResponse{}
	reviewResponse.Allowed = true
	identity := ar.Request.UserInfo.Username
	isInScope := IsInOwnerList(identity)
	ownerLabelValue := OwnerToLabelValue(identity)

	labelValue, hasLabel := obj.ObjectMeta.Labels[labelKey]

	if ar.Request.Operation != v1.Create {
		if isInScope {
			if !hasLabel {
				reviewResponse.Allowed = false
				reviewResponse.Result = &metav1.Status{
					Code: 403,
					Reason: metav1.StatusReason(
						fmt.Sprintf("%s not allowed to %s a resource without the %s label.",
							identity,
							ar.Request.Operation,
							labelKey,
						),
					),
				}
			} else if labelValue != ownerLabelValue {
				reviewResponse.Allowed = false
				reviewResponse.Result = &metav1.Status{
					Code: 403,
					Reason: metav1.StatusReason(
						fmt.Sprintf("%s not allowed to %s a resource with another owner in label %s=%s.",
							identity,
							ar.Request.Operation,
							labelKey,
							labelValue,
						),
					),
				}
			}
		}
		return &reviewResponse
	}

	if !isInScope {
		ownerLabelValue, isInScope, err = getOwnerObjLabel(&obj.ObjectMeta, ar.Request)
		if err != nil {
			klog.Error(err)
			return toV1AdmissionResponse(err)
		}
	}

	// if cloud not get an in-scope owner skip
	if !isInScope {
		return &reviewResponse
	}

	pt := v1.PatchTypeJSONPatch
	switch {
	case len(obj.ObjectMeta.Labels) == 0:
		reviewResponse.Patch = []byte(getFirstLabelPatch(ownerLabelValue))
		reviewResponse.PatchType = &pt
		klog.V(1).Info("patching ", getFirstLabelPatch(ownerLabelValue))
	case !hasLabel:
		reviewResponse.Patch = []byte(getAdditionalLabelPatch(ownerLabelValue))
		reviewResponse.PatchType = &pt
		klog.V(1).Info("patching ", getAdditionalLabelPatch(ownerLabelValue))
	case labelValue != ownerLabelValue:
		reviewResponse.Allowed = false
		reviewResponse.Result = &metav1.Status{
			Code: 403,
			Reason: metav1.StatusReason(
				fmt.Sprintf("%s not allowed to create a resources with another owner in label %s=%s.",
					ar.Request.UserInfo.Username,
					labelKey,
					labelValue,
				),
			),
		}
	default:
		// already set
	}
	return &reviewResponse
}

func getOwnerObjLabel(metadata *metav1.ObjectMeta, request *v1.AdmissionRequest) (string, bool, error) {
	ownerLabelValue := ""
	isInScope := false

	if len(metadata.OwnerReferences) > 0 {
		ownerReferenceLabels, err := getOwnerReferenceLabels(metadata.GetOwnerReferences(), request.Namespace)
		if err != nil {
			return "", isInScope, err
		}

		ownerReferenceLabel := ownerReferenceLabels[labelKey]
		isInScope = IsInOwnerList(OwnerFromLabelValue(ownerReferenceLabel))
		ownerLabelValue = ownerReferenceLabel
	} else if serviceAccountName, ok := metadata.Annotations["kubernetes.io/service-account.name"]; request.Kind.Kind == "Secret" && ok {
		serviceAccountOwnerRef := metav1.OwnerReference{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
			Name:       serviceAccountName,
		}

		ownerReferenceLabels, err := getOwnerReferenceLabels([]metav1.OwnerReference{serviceAccountOwnerRef}, request.Namespace)
		if err != nil {
			return "", isInScope, err
		}

		ownerReferenceLabel := ownerReferenceLabels[labelKey]
		isInScope = IsInOwnerList(OwnerFromLabelValue(ownerReferenceLabel))
		ownerLabelValue = ownerReferenceLabel
	}

	return ownerLabelValue, isInScope, nil
}

var getOwnerReferenceLabels = func(ownerReference []metav1.OwnerReference, namespace string) (map[string]string, error) {
	if len(ownerReference) == 0 {
		return nil, errors.New("no owners found")
	}
	firstOwner := ownerReference[0]

	labels, err := GetObjectLabels(firstOwner.APIVersion, firstOwner.Kind, namespace, firstOwner.Name)

	if err != nil {
		return nil, fmt.Errorf("failed to get owner labels: %s", err)
	}

	return labels, nil
}
