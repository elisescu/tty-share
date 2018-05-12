docker run -v /data/tty_server:/data/ -p 6543:6543 -p 8010:8010 --restart unless-stopped -d --name tty_server  tty_server
