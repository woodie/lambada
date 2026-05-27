# Lambada scan exporter

A minimal SMTP server (written in Go) that accepts incoming email and saves attachments. 
Run this on a **Raspberry Pi** along with **Samba**, to share scans from a scanner that requires and open relay to email scans. 

<img width="697" height="358" alt="flow" src="https://github.com/user-attachments/assets/b7f68dcf-838a-4675-9b15-e6dc7f81e932" />

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

sudo systemctl daemon-reload
sudo systemctl enable lambada
sudo systemctl start lambada
sudo systemctl status lambada
```
