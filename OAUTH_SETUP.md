# Google OAuth Setup For Manual QA

Use these steps to create the Google OAuth client required by G&T's `:add-account` flow.

Do not write the generated client ID or client secret into this file, `.qa/`, `config.toml`, screenshots, or commits. G&T stores them in the OS keychain after you paste them into the TUI.

## Create The Google Cloud Project

1. Open Google Cloud Console:
   <https://console.cloud.google.com/>
2. Use the project selector in the top bar.
3. Click `New Project`.
4. Name it `gandt-manual-qa`.
5. Click `Create`.
6. Confirm `gandt-manual-qa` is the selected project.

## Enable Gmail API

1. Go to `APIs & Services` > `Library`.
2. Search for `Gmail API`.
3. Open `Gmail API`.
4. Click `Enable`.

## Configure OAuth Consent

1. Go to `Google Auth platform` > `Branding`.
2. Click `Get Started` if prompted.
3. Set app name to `G&T QA`.
4. Set user support email to your email address.
5. Choose the audience:
   - Use `External` for regular Gmail test accounts.
   - Use `Internal` only if all test accounts are in your Google Workspace organization.
6. Set contact email to your email address.
7. Accept the Google API Services User Data Policy.
8. Click `Create`.

## Add Test Users

If the OAuth app is in testing mode:

1. Go to `Google Auth platform` > `Audience`.
2. Add the three disposable Gmail QA accounts as test users:
   - Account A
   - Account B
   - Account C
3. Save.

## Create The Desktop OAuth Client

1. Go to `Google Auth platform` > `Clients`.
2. Click `Create Client`.
3. Set application type to `Desktop app`.
4. Name it `G&T local QA`.
5. Click `Create`.
6. Copy the generated `Client ID` and `Client secret`.

Create only one Desktop OAuth client for this QA pass. Reuse the same client ID and client secret for every Gmail account you add to G&T.

## Resume G&T Manual QA

From the project root:

```sh
cd /home/karlo/uwchlan/gandt
export XDG_CONFIG_HOME="$PWD/.qa/config"
export XDG_DATA_HOME="$PWD/.qa/data"
export XDG_DOWNLOAD_DIR="$PWD/.qa/downloads"
./bin/gandt
```

In G&T:

```text
:add-account
```

On the first account, paste the Google Desktop OAuth client ID and client secret when prompted, then complete browser consent for Account A.

For Accounts B and C, run `:add-account` again and complete browser consent. G&T should reuse the same stored OAuth client credentials.

If the keychain is not running in WSL, restart it before launching G&T:

```sh
pkill -u "$USER" -f '[g]nome-keyring-daemon|[g]cr-prompter|[s]ecret-tool' 2>/dev/null || true
read -rsp "Keyring password: " GANDT_KEYRING_PASS
echo
printf '%s\n' "$GANDT_KEYRING_PASS" | gnome-keyring-daemon --unlock
eval "$(gnome-keyring-daemon --start --components=secrets)"
unset GANDT_KEYRING_PASS
```

Optional keyring check:

```sh
printf 'ok' | secret-tool store --label='gandt test' app gandt-test key probe
secret-tool lookup app gandt-test key probe
secret-tool clear app gandt-test key probe
```

Expected lookup output:

```text
ok
```
