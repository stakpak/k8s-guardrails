# K8s Guardrails

>**Warning**
>this project is still under development!


K8s Guardrails POC allowes you to restrict specific user identities (e.g. service account) to only control resources they create to reduce the blast radius of a compormise.

## How it works
1) define in-scope service accounts
2) mutation webhook will add a label `guardrails.guku.io/owner` to all resources directly or indirectly created by the service account to indicate ownership
3) webhooks will allow update or delete requests only to labeled resources
4) service accounts in scope are not allowed to add or modify the owner label

## Caveats
+ without using authn/authz webhooks it is not possible for admission webhooks to restrict access of read actions, and authn/authz webhook require changes to api-server flags which is not possible on many manage Kubernetes services
+ restricted service accounts can create other service accounts and assuming them to bypass the guardrails