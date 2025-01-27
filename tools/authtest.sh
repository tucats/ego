#!/bin/zsh -v
## Default configuration must have logged in within the expiration period for
## this token to be valid.

token="$(ego config ego.logon.token)"

curl -L --cacert lib/https-server.crt https://$HOST.local/services/admin/authenticate/ --oauth2-bearer $token 

