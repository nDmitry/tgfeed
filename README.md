`tgfeed` converts Telegram channels into Atom feeds suitable for any RSS reader of your choice. It runs as a daemon and scrapes t.me channel pages, generating Atom feeds for each channel you put in the config. Resulting feeds can be served with any web server as static files.

## Usage

Running using Docker:

```shell
$ cp config.example.json config.json
# edit config.json to add channels usernames you want to read
$ mkdir cache feeds
$ docker compose up -d
$ # serve the contents of ./feeds folder generated by tgfeed
```

Example nginx config to serve Atom feeds:

```
server {
    listen 80;
    server_name _;

    location /feeds/ {
        alias /var/www/tgfeed/feeds;
        autoindex on;
    }
}
```