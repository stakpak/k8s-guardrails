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
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/klog/v2"
	// TODO: try this library to see if it generates correct json patch
	// https://github.com/mattbaird/jsonpatch
)

func IsInOwnerList(userName string) bool {
	for _, v := range owners {
		if v == userName {
			return true
		}
	}
	return false
}

func server(cmd *cobra.Command, args []string) {
	// certFile, _ := cmd.Flags().GetString("tls-crt-path")
	// keyFile, _ := cmd.Flags().GetString("tls-key-path")
	owners, _ = cmd.Flags().GetStringSlice("owner")

	config := Config{
		CertFile: "/etc/webhook/certs/tls.crt",
		KeyFile:  "/etc/webhook/certs/tls.key",
	}

	http.HandleFunc("/mutate", ServeOnMutate)
	http.HandleFunc("/readyz", func(w http.ResponseWriter, req *http.Request) { w.Write([]byte("ok")) })
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", 8443),
		TLSConfig: configTLS(config),
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		err := server.ListenAndServeTLS("", "")
		if err != nil {
			panic(err)
		}
		wg.Done()
	}()

	time.Sleep(time.Second)

	caCert, err := os.ReadFile(config.CertFile)
	if err != nil {
		panic(err)
	}
	createMutationConfig(caCert)

	wg.Wait()
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "webhook",
		Short: "Starts a HTTP server, useful for testing MutatingAdmissionWebhook and ValidatingAdmissionWebhook",
		Long: `Starts a HTTP server, useful for testing MutatingAdmissionWebhook and ValidatingAdmissionWebhook.
	After deploying it to Kubernetes cluster, the Administrator needs to create a ValidatingWebhookConfiguration
	in the Kubernetes cluster to register remote webhook admission controllers.`,
		Args: cobra.MaximumNArgs(0),
		Run:  server,
	}

	rootCmd.Flags().SortFlags = false
	klog.InitFlags(nil)

	pflag.CommandLine.AddGoFlag(flag.CommandLine.Lookup("v"))

	// rootCmd.Flags().String("tls-crt-path", "",
	// 	"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
	// rootCmd.MarkFlagRequired("tls-crt-path")
	// rootCmd.Flags().String("tls-key-path", "",
	// 	"File containing the default x509 private key matching --tls-crt-path.")
	// rootCmd.MarkFlagRequired("tls-key-path")
	rootCmd.Flags().StringSlice("owner", []string{},
		"One or multiple owners <namespace>:<name> in scope, e.g. --owner system:serviceaccount:guku:guku .")
	rootCmd.MarkFlagRequired("owner")

	rootCmd.Execute()
}
