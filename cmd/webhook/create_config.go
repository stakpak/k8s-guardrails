package main

import (
	"context"
	"os"
	"strconv"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
)

func createMutationConfig(caCert []byte) {

	var (
		webhookNamespace, _ = os.LookupEnv("WEBHOOK_NAMESPACE")
		webhookService, _   = os.LookupEnv("WEBHOOK_SERVICE")
		webhookPath, _      = os.LookupEnv("WEBHOOK_PATH")
		webhookPort, _      = os.LookupEnv("WEBHOOK_PORT")
		mutationCfgName, _  = os.LookupEnv("MUTATE_CONFIG")
	)

	webhookPortInt, _ := strconv.ParseInt(webhookPort, 10, 32)
	webhookPortInt32 := int32(webhookPortInt)

	ctx := context.Background()
	config := ctrl.GetConfigOrDie()
	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic("failed to set go -client")
	}

	// fail := admissionregistrationv1.Fail
	scope := admissionregistrationv1.AllScopes
	sideEffect := admissionregistrationv1.SideEffectClassNone

	mutateconfig := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: mutationCfgName,
		},
		Webhooks: []admissionregistrationv1.MutatingWebhook{{
			Name: mutationCfgName,
			AdmissionReviewVersions: []string{
				"v1",
				"v1beta1",
			},
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: caCert,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      webhookService,
					Namespace: webhookNamespace,
					Path:      &webhookPath,
					Port:      &webhookPortInt32,
				},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{
					Operations: []admissionregistrationv1.OperationType{
						admissionregistrationv1.Create,
						admissionregistrationv1.Delete,
						admissionregistrationv1.Update,
					},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"*"},
						APIVersions: []string{"*"},
						Resources:   []string{"*"},
						Scope:       &scope,
					},
				}},
			// FailurePolicy: &fail,
			SideEffects: &sideEffect,
		}},
	}

	klog.V(2).Info("creating mutation webhook config")

	if _, err := kubeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(
		ctx,
		mutateconfig,
		metav1.CreateOptions{},
	); err != nil {
		panic(err)
	}
}
