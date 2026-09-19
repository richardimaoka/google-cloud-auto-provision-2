
Files marked (TODO) have not been recorded yet.

## Method: groups.get

GET https://cloudidentity.googleapis.com/v1/{name=groups/*}
https://docs.cloud.google.com/identity/docs/reference/rest/v1/groups/get

groups.get.json

## Method: groups.memberships.list

GET https://cloudidentity.googleapis.com/v1/{parent=groups/*}/memberships
https://docs.cloud.google.com/identity/docs/reference/rest/v1/groups.memberships/list

groups.memberships.list.json

## Method: projects.create

POST https://cloudresourcemanager.googleapis.com/v3/projects
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/create

project.create.json
project.create.already_exists.json (409 ALREADY_EXISTS, when the project ID is already taken)
project.create.invalid_argument.json (TODO: 400 INVALID_ARGUMENT, for a malformed project ID or a restricted word such as "google")
project.create.permission_denied.json (TODO: 403, when the caller is not allowed to create projects)

Where a project lands depends on who creates it, because Create sends no parent.
See the operations.get files named caller_*.

## Method: operations.get

GET https://cloudresourcemanager.googleapis.com/v3/{name=operations/**}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/operations/get

operations.get.json
operations.get.running.json (TODO: the operation is not done yet, so `done` is absent)
operations.get.failed.json (TODO: the operation finished with an `error`, for example quota exceeded)
operations.get.delete_done.json (TODO: a finished delete operation)

Finished create operations, from creating a project with no parent as callers in different domains. Compare `parent` in `response`:

operations.get.caller_mydomain_com.json (TODO: caller @mydomain.com, whose domain has an organization)
operations.get.caller_google_com.json (TODO: caller @google.com, a different organization)
operations.get.caller_no_organization.json (TODO: a caller whose domain has no organization, so `response` should have no `parent`)

## Method: projects.get

GET https://cloudresourcemanager.googleapis.com/v3/{name=projects/*}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/get

project.get.json (TODO: an ACTIVE project)
project.get.delete_requested.json (TODO: a project after it was deleted, state DELETE_REQUESTED)
project.get.not_found.json (TODO: a project ID that does not exist. The docs don't say whether this is 404 or 403)
project.get.permission_denied.json (TODO: a project ID owned by someone else, to compare with not_found)

## Method: projects.delete

DELETE https://cloudresourcemanager.googleapis.com/v3/{name=projects/*}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/projects/delete

project.delete.json (TODO: the delete was accepted, and the operation is not done yet)
project.delete.not_found.json (TODO: deleting a project that does not exist)
project.delete.already_deleted.json (TODO: deleting a project that is already DELETE_REQUESTED)
