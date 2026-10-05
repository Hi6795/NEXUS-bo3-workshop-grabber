# Contributing

Thanks for helping improve NEXUS BO3 Workshop Grabber.

## Bug reports

Please include:

- Windows version
- Workshop item URL or numeric ID
- Whether the item is a map, mod, or other content
- Relevant lines from the Activity log
- Whether a retry succeeded

Do not post Steam passwords, session tokens, cookies, or other credentials.

## Pull requests

1. Fork the repository.
2. Create a focused branch.
3. Keep changes scoped and preserve existing behavior unless the PR intentionally changes it.
4. Run:

```bash
go test ./...
go vet ./...
```

5. For Windows UI or SteamCMD changes, include a short description of hardware testing when possible.

## Project boundaries

Please keep the Grabber independent from PS4 porters, Sony SDK material, and proprietary game files.
