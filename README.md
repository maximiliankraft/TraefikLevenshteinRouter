# Traefik Levenshtein Router

A Traefik middleware plugin that redirects a misspelled request hostname to the closest configured canonical hostname.

For example, with `foo.localhost` and `bar.localhost` configured, a request to `fo.localhost` is redirected to `foo.localhost`, not `bar.localhost`.

## Configuration

```yaml
http:
  middlewares:
    levenshtein-router:
      plugin:
        levenshteinrouter:
          hosts:
            - foo.localhost
            - bar.localhost
          maxDistance: 2
          redirectCode: 308
```

`hosts` is required. `maxDistance` defaults to `2`, and `redirectCode` defaults to `308`. The plugin only redirects when a candidate is within the configured edit distance. Exact matches and requests without a suitable candidate are passed to the next handler.

The middleware needs to be attached to a catch-all Traefik router. A router with exact `Host(...)` rules should have a higher priority so canonical requests go directly to their services.

## Local test setup

The `test/` directory contains a complete local Traefik setup with `foo.localhost`, `bar.localhost`, and a catch-all typo router:

```sh
sudo nerdctl compose -f test/compose.yml up -d

# fo.localhost redirects to foo.localhost on the published test port.
curl -i -H 'Host: fo.localhost:18080' http://127.0.0.1:18080/

# Follow the redirect and see the foo response.
curl -i -L \
  --resolve fo.localhost:18080:127.0.0.1 \
  --resolve foo.localhost:18080:127.0.0.1 \
  http://fo.localhost:18080/

sudo nerdctl compose -f test/compose.yml down
```

The local test uses Traefik's `experimental.localPlugins` loader, so the repository is mounted into the expected local plugin source path. The module path in `go.mod`, `.traefik.yml`, and `test/traefik.yml` must stay in sync.
