`tgfeed` converts Telegram channels into RSS or Atom feeds suitable for any RSS reader of your choice. It runs as an HTTP server that dynamically scrapes t.me channel pages and generates RSS or Atom feeds on demand.

## Usage

Running using Docker:

```shell
$ docker compose up -d
```

This will start the tgfeed server on port 8080 (can be changed via HTTP_SERVER_PORT environment variable) and a Redis instance for caching.

## API Endpoints

### Get Channel Feed

```
GET /telegram/channel/{username}
```

Generates a feed for the specified Telegram channel.

#### Path Parameters

- `username` - Telegram channel username (required)

#### Query Parameters

- `format` - Feed format, either "rss" or "atom" (default: "rss")
- `exclude` - List of words to exclude posts containing them, separated by `|` (optional)
- `exclude_case_sensitive` - Whether to match excluded words case-sensitively, "1" or "true" for case-sensitive (default: false)
- `cache_ttl` - Cache TTL in minutes, 0 to disable caching (default: 60)

#### Example

```
# Get RSS feed for "durov" channel
http://localhost:8080/telegram/channel/durov

# Get Atom feed for "durov" channel with exclusions
http://localhost:8080/telegram/channel/durov?format=atom&exclude=crypto|bitcoin

# Get RSS feed with no caching
http://localhost:8080/telegram/channel/durov?cache_ttl=0
```

## Example RSS Reader Configuration

When adding a feed to your RSS reader, use the URL:

```
http://your-server:8080/telegram/channel/channelname
```

Replace `channelname` with the username of the Telegram channel you want to follow.

## Docker Compose

The service is preconfigured with Redis for caching. You can customize the configuration through environment variables in the `compose.yaml` file.
