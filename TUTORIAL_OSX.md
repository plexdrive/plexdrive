# Tutorial for creating/mounting an encrypted rclone volume on OSX

## Install the dependencies
With homebrew installed:
```
brew install mongodb
brew install osxfuse
```
## Mounting the unencrypted volume with plexdrive
1. Download the latest plexdrive release from the release page (as of writing, latest stable is 4.0.0)
```
curl -0 https://github.com/dweidenfeld/plexdrive/releases/download/4.0.0/plexdrive-darwin-10.6-amd64 -o plexdrive
```
2. Move the plexdrive executable to `/usr/bin/`
```
mv plexdrive /usr/bin/plexdrive
```
3. Test that it works
```
plexdrive
```
4. Configure with your Google Developers account, by creating your own client id and client secret (see [https://rclone.org/drive/#making-your-own-client-id](https://rclone.org/drive/#making-your-own-client-id)).
5. Create a launch daemon for automatic startup on boot
```
# /System/Library/LaunchDaemons/plexdrive.plist
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>plexdrive</string>
    <key>Program</key>
    <string>/usr/bin/plexdrive</string>
    <key>ProgramArguments</key>
    <array>
      <string>-v</string>
      <string>2</string>
      <string>/mnt/plexdrive</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
  </dict>
</plist>
```
5. Run the application like this
```
sudo launchctl start plexdrive
```

## Preparations in rclone
1. Download and install rclone
```
curl -O https://downloads.rclone.org/rclone-current-osx-amd64.zip -o rclone.zip
unzip rclone.zip
mv rclone-current-osx-amd64.zip/rclone /usr/bin/rclone
```
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
11. Create a launchd startup script for automatic startup on boot
```
# /System/Library/LaunchDaemons/rclone.plist

<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>rclone</string>
    <key>Program</key>
    <string>/usr/bin/rclone</string>
    <key>ProgramArguments</key>
    <array>
      <string>mount</string>
      <string>--allow-other</string>
      <string>local-crypt:/mnt/media</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
  </dict>
</plist>
```
13. Run the application like this
```
sudo launchctl start rclone
```
