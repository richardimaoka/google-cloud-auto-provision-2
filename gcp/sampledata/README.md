
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

- [`project.create.json`](./project.create.json)
- [`project.create.already_exists.json`](./project.create.already_exists.json) (409 ALREADY_EXISTS, when the project ID is already taken)
- [`project.create.invalid_argument.json`](./README.md) (TODO: 400 INVALID_ARGUMENT, for a malformed project ID or a restricted word such as "google")
- [`project.create.permission_denied.json`](./README.md) (TODO: 403, when the caller is not allowed to create projects)

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

- [`project.get.json`](./README.md) (TODO: an ACTIVE project)
- [`project.get.delete_requested.json`](./README.md) (TODO: a project after it was deleted, state DELETE_REQUESTED)
- [`project.get.not_found.json`](./README.md) (TODO: a project ID that does not exist. The docs don't say whether this is 404 or 403)
- [`project.get.permission_denied.json`](./README.md) (TODO: a project ID owned by someone else, to compare with not_found)

## Method: projects.delete

```
DELETE https://cloudresourcemanager.googleapis.com/v3/{name=projects/*}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/delete
```

- [`project.delete.json`](./README.md) (TODO: the delete was accepted, and the operation is not done yet)
- [`project.delete.not_found.json`](./README.md) (TODO: deleting a project that does not exist)
- [`project.delete.already_deleted.json`](./README.md) (TODO: deleting a project that is already DELETE_REQUESTED)

## To consider: refreshing the samples

The shapes of the API responses may change over time. One way to keep the samples current would be a separate set of scripts that call the APIs and rewrite the sample files, with the identifiers replaced by placeholders.

This is better as scripts than as part of the integration tests. An integration test that covers every sample would create many real projects, which use up the project quota, and their IDs can never be reused. The integration test (`projects_integration_test.go`) runs only the project lifecycle, once. Scripts could be run whenever needed, and each sample could be refreshed on its own.

Not decided whether to build them.
