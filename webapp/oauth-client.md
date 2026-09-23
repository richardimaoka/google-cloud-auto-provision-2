# Setting up the OAuth client

Terraform can't create an OAuth client for a regular web app, so this is done
by hand in the Cloud Console, in the project that `terraform/` manages. The
result is a `client_secret.json` file in `secrets/` under the repository
root, holding the client ID, secret and redirect URI.

The Console menus have been reorganized under **Google Auth Platform**, and the
names may change again. Open the project, then search for "Google Auth
Platform" in the top search bar.

## 1. Branding

Under **Branding**, set what users see on the sign-in and consent screens:

- **App name**
- **User support email**
- **Developer contact email**

If Google Auth Platform shows a **Get started** button, use it. It walks
through branding and audience together.

## 2. Audience

Under **Audience**, choose who can sign in:

- **Internal:** only members of your Google Cloud organization. This needs the
  project to be inside an organization, and is the simplest choice if everyone
  who will sign in belongs to it.
- **External:** any Google account. While the app's publishing status is
  **Testing**, only the accounts listed as **Test users** can sign in, so add
  your own account there.

## 3. Data access (scopes)

Under **Data Access**, add the scopes the app requests. The token helper in
`webapp/oauth_token_test.go` requests:

- `https://www.googleapis.com/auth/cloud-platform`
- `https://www.googleapis.com/auth/userinfo.email`

Keep this list in sync with the `scopes` variable in that file.

## 4. Create the client

Under **Clients**, choose **Create client** and fill in:

- **Application type:** Web application
- **Name:** anything, for example `auto-provision`
- **Authorized redirect URIs:** for example `http://localhost:8080/oauth2/callback`

The redirect URI must match exactly, including the port and path. The token
helper uses the first redirect URI in the file, and only accepts a `localhost`
or `127.0.0.1` one.

## 5. Download the client file

When you create the client, use **Download JSON** and save the file as
`secrets/client_secret.json` under the repository root (create the `secrets/`
directory if it doesn't exist yet). Google's documentation says the secret
can't be recovered later, so download it then. If you lose it, you have to
create a new client.

`client_secret.json` is git-ignored. Never commit it. If it leaks, delete the
client in the Console and create a new one.

## 6. Check that it works

From the repository root:

```sh
go test -tags oauthtoken -count=1 -v ./webapp
```

Open the URL it prints, approve access, and the token is saved to
`webapp/testdata/token.json`, which is git-ignored.

## Notes

- **Refresh token lifetime in Testing:** as I understand it, for an External app
  in Testing status, refresh tokens expire after about 7 days, and you would
  have to authorize again. Google's setup page doesn't cover this, so check
  Google's current OAuth documentation before relying on long-lived tokens.
  An Internal app doesn't have this limit.
- **Unverified apps:** an External app requesting sensitive scopes shows an
  "unverified app" warning until it goes through the Verification Center. That's
  fine for you and your test users.
