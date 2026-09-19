
Files marked (TODO) have not been recorded yet.

## File names

`<method>.success.json` or `<method>.error.json`. A method with more than one file of a kind adds a case name to that kind: `<method>.success.<case>.json`, `<method>.error.<case>.json`. Successes and errors are counted separately.

`success` is a normal response. `error` is an error response, or an operation that finished with an `error`.

## Placeholders

A recorded response holds real identifiers. `go run ./cmd/anonymize gcp/samples/scripts/out/<name>.json` replaces them and writes the file here. It replaces:

| Real value | Placeholder |
|---|---|
| Project number in a normal response | `projects/123456789012`, then `234567890123`, `345678901234` for more |
| Project number in an error response (a project the caller cannot use) | `projects/987654321098`, then `876543210987`, `765432109876` |
| Organization | `organizations/100000000000`, then `200000000000`, `300000000000` |
| Project ID (`projectId`, and a violation's `subject`), everywhere it appears | `my-project-123`, then `my-project-456`, `my-project-789` |
| Project ID in an error response that is not a `projectId` (`projects/<ID>`, a project the caller cannot use or that does not exist) | `this-does-not-exist-1`, then `this-does-not-exist-2`, `this-does-not-exist-3` |
| Number of an operation | `operations/dp.1234567890123456789` and the like |
| `etag` of a project | `W/"0123456789abcdef"` |
| `etag` of an IAM policy (base64, no `W/`) | `BwAAAAAAAAA=` |
| Project number in a service account (`service-<n>@…`, `<n>-compute@…`, `<n>@cloudservices…`) | the same placeholder as `projects/<n>` |
| Project ID in a service account (`<name>@<project ID>.iam.gserviceaccount.com`, `<project ID>@appspot…`) and in `projectOwner:`, `projectEditor:`, `projectViewer:` | `my-project-123` and the like, as above |
| Name of a service account you made (`<name>@<project ID>.iam…`) | `my-service-account`, then `my-service-account-2`, `my-service-account-3` |
| `user:` member | `user:alice@example.com`, then `bob@`, `carol@`, `dave@` |
| `group:` member | `group:group-1@example.com`, then `group-2@`, `group-3@` |
| `domain:` member | `domain:example.com` |
| Error ID (base64, holds a request UUID and the resource) | UUID `00000000-0000-0000-0000-000000000000`, and the resource replaced as above |

A service agent (`service-<n>@<agent>.iam.gserviceaccount.com`) keeps its domain, which belongs to Google. `allUsers` and `allAuthenticatedUsers` stay as they are.

The same real value gets the same placeholder within a file. Times, and the names of projects made up not to exist (they start with `this-does-not-exist`), stay as they are. The tool stops, and writes nothing, if an email address that is not `@example.com` is left (the service account addresses above are the exception), an error ID has a layout it doesn't know, or an IAM binding has a `condition`, a member such as `deleted:` or `principal://`, or a service account in a form it doesn't know. Replace those by hand.

The groups samples were made by hand: `example.com` addresses, `groups/0123456789abcde`, `customers/C0123abcd`. The tool does not handle them.

## Method: groups.get

```
GET https://cloudidentity.googleapis.com/v1/{name=groups/*}
https://docs.cloud.google.com/identity/docs/reference/rest/v1/groups/get
```

- [`groups.get.success.json`](./groups.get.success.json)

## Method: groups.memberships.list

```
GET https://cloudidentity.googleapis.com/v1/{parent=groups/*}/memberships
https://docs.cloud.google.com/identity/docs/reference/rest/v1/groups.memberships/list
```

- [`groups.memberships.list.success.json`](./groups.memberships.list.success.json)

## Method: projects.create

```
POST https://cloudresourcemanager.googleapis.com/v3/projects
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/create
```

- [`projects.create.success.json`](./projects.create.success.json) (the create request was accepted, and the operation is not done yet)
- [`projects.create.error.already_exists.json`](./projects.create.error.already_exists.json) (409 ALREADY_EXISTS, when the project ID is already taken)
- [`projects.create.error.invalid_argument.json`](./README.md) (TODO: 400 INVALID_ARGUMENT, for a malformed project ID or a restricted word such as "google")
- [`projects.create.error.permission_denied.json`](./README.md) (TODO: 403 PERMISSION_DENIED, when the caller is not allowed to create projects)

Where a project lands depends on who creates it, because Create sends no parent.
See the operations.get files named caller_*.

## Method: operations.get

```
GET https://cloudresourcemanager.googleapis.com/v3/{name=operations/**}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/operations/get
```

- [`operations.get.success.create_done.json`](./operations.get.success.create_done.json) (a finished create operation)
- [`operations.get.success.running.json`](./README.md) (TODO: the operation is not done yet, so `done` is absent)
- [`operations.get.success.delete_done.json`](./operations.get.success.delete_done.json) (a finished delete operation. `response` is the project, now in state DELETE_REQUESTED)
- [`operations.get.error.json`](./README.md) (TODO: the operation finished with an `error`, for example quota exceeded)

Finished create operations, from creating a project with no parent as callers in different domains. Compare `parent` in `response`:

- [`operations.get.success.caller_mydomain_com.json`](./README.md) (TODO: caller @mydomain.com, whose domain has an organization)
- [`operations.get.success.caller_google_com.json`](./README.md) (TODO: caller @google.com, a different organization)
- [`operations.get.success.caller_no_organization.json`](./README.md) (TODO: a caller whose domain has no organization, so `response` should have no `parent`)

## Method: projects.get

```
GET https://cloudresourcemanager.googleapis.com/v3/{name=projects/*}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/get
```

- [`projects.get.success.with_parent.json`](./projects.get.success.with_parent.json) (an ACTIVE project. `name` is `projects/<project number>`, not the project ID, which is in `projectId`)
- [`projects.get.success.no_parent.json`](./README.md) (TODO: a project with no parent, so `parent` is absent)
- [`projects.get.success.delete_requested.json`](./projects.get.success.delete_requested.json) (a project after it was deleted, state DELETE_REQUESTED, with `deleteTime`. It is still a normal response)
- [`projects.get.error.invalid_argument.json`](./projects.get.error.invalid_argument.json) (400 INVALID_ARGUMENT, for a malformed project ID)
- [`projects.get.error.non_existent.json`](./projects.get.error.non_existent.json) (403 PERMISSION_DENIED, for a project ID that does not exist)
- [`projects.get.error.permission_denied.json`](./projects.get.error.permission_denied.json) (403 PERMISSION_DENIED, for a project owned by someone else)

A project that does not exist returns 403, not 404. The API does not tell "not allowed" from "does not exist": both are the same 403 PERMISSION_DENIED, and the message says "(or it may not exist)". Only the resource and the error ID differ between non_existent and permission_denied.

## Method: projects.delete

```
DELETE https://cloudresourcemanager.googleapis.com/v3/{name=projects/*}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/delete
```

- [`projects.delete.success.json`](./README.md) (TODO: the delete was accepted, and the operation is not done yet)
- [`projects.delete.error.non_existent.json`](./README.md) (TODO: deleting a project that does not exist. Probably 403 like Get, not confirmed)
- [`projects.delete.error.permission_denied.json`](./README.md) (TODO: deleting a project owned by someone else)
- [`projects.delete.error.already_deleted.json`](./projects.delete.error.already_deleted.json) (400 FAILED_PRECONDITION "Project not active", when deleting a project that is already DELETE_REQUESTED)

## To consider: refreshing the samples

The shapes of the API responses may change over time. One way to keep the samples current would be a separate set of scripts that call the APIs and rewrite the sample files, with the identifiers replaced by placeholders.

This is better as scripts than as part of the integration tests. An integration test that covers every sample would create many real projects, which use up the project quota, and their IDs can never be reused. The integration test (`projects_integration_test.go`) runs only the project lifecycle, once. Scripts could be run whenever needed, and each sample could be refreshed on its own.

Not decided whether to build them.

## Method: projects.getIamPolicy

```
POST https://cloudresourcemanager.googleapis.com/v3/{resource=projects/*}:getIamPolicy
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/getIamPolicy
```

- [`projects.getIamPolicy.success.json`](./projects.getIamPolicy.success.json) (the policy of a project: `bindings` hold one role with its `members`. Members are service accounts, service agents and a user)
- [`projects.getIamPolicy.error.json`](./projects.getIamPolicy.error.json) (403 PERMISSION_DENIED, for a project the caller cannot use or that does not exist. The API does not tell the two apart)
