# Recording API samples

One script per sample in `gcp/sampledata`. Each script shows the `curl` request, and saves the response to `out/<sample name>.json`.

`out/` is git-ignored, because the responses hold real identifiers. Replace them with placeholders before a file goes into `gcp/sampledata`.

## Usage

Log in with the account you want to call as, then run a script:

```sh
gcloud auth login            # once per account, see: gcloud auth list
ACCOUNT=me@example.com QUOTA_PROJECT=my-quota-project \
  ./projects.get.error.non_existent.sh
```

- `ACCOUNT` is a gcloud account. The script says who it calls as, so the response can be tied to that account.
- `QUOTA_PROJECT` is the project the API calls are counted against. It is sent as `x-goog-user-project`.
- Scripts that need a project or an operation take it from a variable, named in the script. `PROJECT` is a project number or ID.

## Scripts

| Script | Extra variable | Safe? |
|---|---|---|
| `projects.get.error.invalid_argument.sh` | | read only |
| `projects.get.error.non_existent.sh` | | read only |
| `projects.get.error.permission_denied.sh` | `PROJECT` (owned by someone else) | read only |
| `projects.get.success.with_parent.sh` | `PROJECT` (ACTIVE, has a parent) | read only |
| `projects.get.success.delete_requested.sh` | `PROJECT` (deleted) | read only |
| `operations.get.success.delete_done.sh` | `OPERATION` | read only |
| `projects.create.error.invalid_argument.sh` | | rejected, creates nothing |
| `projects.create.error.already_exists.sh` | `EXISTING_PROJECT_ID` | rejected, creates nothing |
| `projects.delete.error.non_existent.sh` | | random ID, deletes nothing |
| `projects.delete.error.already_deleted.sh` | `PROJECT` (deleted) | refuses unless the project is DELETE_REQUESTED |

## Not scripted

These would create or delete real projects, so they are left out for now:

- `projects.create.error.permission_denied`: it needs an account that may not create projects. A valid request from an account that may would create a project.
- `operations.get.success.running`, `operations.get.error` and `projects.delete.success`: they need a project to be created or deleted. Take them from a lifecycle run.
- `operations.get.success.caller_*` and `projects.get.success.no_parent`: they create a project as another account.
- `projects.delete.error.permission_denied`: it deletes a project owned by someone else. If the account is allowed, it is deleted.
