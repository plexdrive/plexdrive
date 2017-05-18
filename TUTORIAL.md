# Tutorial for creating/mounting an encrypted rclone volume

## Install the dependencies
You have to install fuse on your system to run plexdrive/rclone. Please check your system on how to install fuse. 
Normally you can use:
```
apt-get install fuse
```

## Mounting the unencrypted volume with plexdrive
1. Then you should download the newest release from the [GitHub release page](https://github.com/dweidenfeld/plexdrive/releases).
2. Create your own client id and client secret (see [https://rclone.org/drive/#making-your-own-client-id](https://rclone.org/drive/#making-your-own-client-id)).
3. Create a systemd startup script for automatic startup on boot
```
# /etc/systemd/system/plexdrive.service

[Unit]
Description=Plexdrive
AssertPathIsDirectory=/mnt/plexdrive
After=network-online.target

[Service]
Type=simple
ExecStart=/usr/bin/plexdrive -v 2 /mnt/plexdrive
ExecStop=/bin/fusermount -u /mnt/plexdrive
Restart=on-abort

[Install]
WantedBy=default.target
```
4. Refresh your daemons
```
sudo systemctl daemon-reload
```
5. Run the application like this
```
sudo systemctl start plexdrive.service
```
6. Activate the auto startup option
```
sudo systemctl enable plexdrive.service
```

## Preparations in rclone
1. Download and install rclone
2. Configure a new rclone remote:
```
rclone config
```
3. Select "new remote"
![remote image](http://i.imgur.com/nOg64dy.png)
3. Give the remote a descriptive name. We will be using the name "local-crypt" throughout the rest of this guide.
4. Select "5" for "Encrypt/Decrypt a remote"
![type image](http://i.imgur.com/bLtWR7P.png)
5. Now we need to specify the remote to decrypt. This needs to be the path where plexdrive is mounted:
```
/mnt/plexdrive/encrypted
```
6. Encryption type: Select the same type of encryption that you initially chose when setting up your rclone encryption.
7. Password: Use the same password you used then setting up your rclone encryption.
8. Salt: Use the same salt you used when setting up your rclone encryption.
9. Review the details and if everything looks good select "y".
10. We should now have a working Encrypt/Decrypt remote.
11. Create a systemd startup script for automatic startup on boot
```
# /etc/systemd/system/rclone.service

[Unit]
Description=Google Drive (rclone)
AssertPathIsDirectory=/mnt/media
After=plexdrive.service

[Service]
Type=simple
ExecStart=/usr/bin/rclone mount --allow-other local-crypt: /mnt/media
ExecStop=/bin/fusermount -u /mnt/media
Restart=on-abort

[Install]
WantedBy=default.target
```
12. Refresh your daemons
```
sudo systemctl daemon-reload
```
13. Run the application like this
```
sudo systemctl start rclone.service
```
14. Activate the auto startup option
```
sudo systemctl enable rclone.service
```