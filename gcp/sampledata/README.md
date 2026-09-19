
Files marked (TODO) have not been recorded yet.

## Method: groups.get

```
GET https://cloudidentity.googleapis.com/v1/{name=groups/*}
https://docs.cloud.google.com/identity/docs/reference/rest/v1/groups/get
```

- [`groups.get.json`](./groups.get.json)

## Method: groups.memberships.list

```
GET https://cloudidentity.googleapis.com/v1/{parent=groups/*}/memberships
https://docs.cloud.google.com/identity/docs/reference/rest/v1/groups.memberships/list
```

- [`groups.memberships.list.json`](./groups.memberships.list.json)

## Method: projects.create

```
POST https://cloudresourcemanager.googleapis.com/v3/projects
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/create
```

- [`projects.create.success.json`](./projects.create.success.json) (the create request was accepted, and the operation is not done yet)
- [`projects.create.already_exists.json`](./projects.create.already_exists.json) (409 ALREADY_EXISTS, when the project ID is already taken)
- [`projects.get.invalid_argument.json`](./projects.get.invalid_argument.json) (400 INVALID_ARGUMENT. Not recorded for create, the error has the same shape. The docs say create returns it for a malformed project ID or a restricted word such as "google")
- [`projects.get.permission_denied.json`](./projects.get.permission_denied.json) (403 PERMISSION_DENIED. Not recorded for create, the error has the same shape, for a caller who is not allowed to create projects)

Where a project lands depends on who creates it, because Create sends no parent.
See the operations.get files named caller_*.

## Method: operations.get

```
GET https://cloudresourcemanager.googleapis.com/v3/{name=operations/**}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/operations/get
```

- [`operations.get.json`](./operations.get.json)
- [`operations.get.running.json`](./README.md) (TODO: the operation is not done yet, so `done` is absent)
- [`operations.get.failed.json`](./README.md) (TODO: the operation finished with an `error`, for example quota exceeded)
- [`operations.get.delete_done.json`](./README.md) (TODO: a finished delete operation)

Finished create operations, from creating a project with no parent as callers in different domains. Compare `parent` in `response`:

- [`operations.get.caller_mydomain_com.json`](./README.md) (TODO: caller @mydomain.com, whose domain has an organization)
- [`operations.get.caller_google_com.json`](./README.md) (TODO: caller @google.com, a different organization)
- [`operations.get.caller_no_organization.json`](./README.md) (TODO: a caller whose domain has no organization, so `response` should have no `parent`)

## Method: projects.get

```
GET https://cloudresourcemanager.googleapis.com/v3/{name=projects/*}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/get
```

- [`projects.get.success.json`](./projects.get.success.json) (an ACTIVE project. `name` is `projects/<project number>`, not the project ID, which is in `projectId`)
- [`projects.get.delete_requested.json`](./README.md) (TODO: a project after it was deleted, state DELETE_REQUESTED)
- [`projects.get.invalid_argument.json`](./projects.get.invalid_argument.json) (400 INVALID_ARGUMENT, for a malformed project ID)
- [`projects.get.non_existent.json`](./projects.get.non_existent.json) (403 PERMISSION_DENIED, for a project ID that does not exist)
- [`projects.get.permission_denied.json`](./projects.get.permission_denied.json) (403 PERMISSION_DENIED, for a project owned by someone else)

A project that does not exist returns 403, not 404. The API does not tell "not allowed" from "does not exist": both are the same 403 PERMISSION_DENIED, and the message says "(or it may not exist)". Only the resource and the error ID differ between non_existent and permission_denied.

## Method: projects.delete

```
DELETE https://cloudresourcemanager.googleapis.com/v3/{name=projects/*}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/delete
```

- [`projects.delete.json`](./README.md) (TODO: the delete was accepted, and the operation is not done yet)
- [`projects.delete.non_existent.json`](./README.md) (TODO: deleting a project that does not exist. Probably 403 like Get, not confirmed)
- [`projects.delete.already_deleted.json`](./README.md) (TODO: deleting a project that is already DELETE_REQUESTED)

## To consider: refreshing the samples

The shapes of the API responses may change over time. One way to keep the samples current would be a separate set of scripts that call the APIs and rewrite the sample files, with the identifiers replaced by placeholders.

This is better as scripts than as part of the integration tests. An integration test that covers every sample would create many real projects, which use up the project quota, and their IDs can never be reused. The integration test (`projects_integration_test.go`) runs only the project lifecycle, once. Scripts could be run whenever needed, and each sample could be refreshed on its own.

Not decided whether to build them.
