# Lambada scan exporter

A minimal SMTP server written in Go that accepts incoming email and saves all attachments.

There attachments can then be shared via Samba.

### Setup

Make a symbolic link to Samba's public folder.
```
ln -s /srv/samba/public attachments
```

Compile lambada (after installing go).
```
install github.com/emersion/go-smtp@latest
go build 
```

### Install the service
 91 ```
 92 sudo cp system/lambada.service /etc/systemd/system/
 93 # Remember to edit the username
 94 sudo chmod a+rwx /etc/systemd/system/lambada.service
 95 
 96 sudo systemctl enable lambada
 97 sudo systemctl start lambada
 98 ```
