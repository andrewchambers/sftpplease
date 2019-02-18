# sftpplease

This program acts as a bridge between an ssh server an supported cloud providers.
This lets you use ssh to access dropbox without installing any closed source software
on your devices :). 

# How to use:

Download or compile sftpplease, install it on your server.

Create a user account on your server, (for example a dropbox user).

Create an authorized keys file with a forced command:

```
restrict,command="/path/to/sftpplease -read-only -vfs dropbox:YOUR_API_TOKEN", ssh-rsa KEYKEYKEYKEY...
```

Now you can use sftp,sshfs and scp to access your dropbox account :).



## Currently supported providers

## Dropbox

### Creating a dropbox api token

- Visit https://www.dropbox.com/developers
- Go to 'My apps' (https://www.dropbox.com/developers/apps)
- Go to create app (https://www.dropbox.com/developers/apps/create)
- Follow the guides.
- Visit your app page.
- Click the 'Generate' bitton to generate an api access token, Use this token under -the 'vfs dropbox:YOUR_API_TOKEN' argument.
