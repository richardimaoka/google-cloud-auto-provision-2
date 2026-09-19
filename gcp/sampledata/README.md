
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

## Method: operations.get

GET https://cloudresourcemanager.googleapis.com/v3/{name=operations/**}
https://docs.cloud.google.com/resource-manager/reference/rest/v3/operations/get

operations.get.json
