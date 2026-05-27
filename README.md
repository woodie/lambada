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

You need to edit the username in the service file.
```
sudo cp system/lambada.service /etc/systemd/system/
sudo chmod a+rwx /etc/systemd/system/lambada.service

sudo systemctl enable lambada
sudo systemctl start lambada
```
