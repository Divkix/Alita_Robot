# Panel initData fixtures

`initdata_main.txt` is a published real-client payload copied verbatim from the
`telegram-mini-apps/init-data-golang` README example
(https://github.com/Telegram-Mini-Apps/init-data-golang, README.md lines 31 and
34 at commit 9da1e4e976acc4be94698045c4b531c8508764ae). It was not captured in this repository's test
environment; it is the library's documented example, and the payload carries
the `signature` field current Telegram clients send. The bot token that signed
it is the library's published example token:

    7342037359:AAHI25ES9xCOMPokpYoz-p8XVrZUdygo2J4

`initdata_special_name.txt` is synthetic. It was generated out of band with an
independent Python 3 implementation of Telegram's documented HMAC procedure
(not with this package) so that `first_name` is `A&B=C`, URL-encoded in the
payload, and `start_param` is `chat_-1001234567890`. It is not a live-client
capture.

Both payloads use `auth_date=1733509682` so the tests can pass a fixed clock.
They remain valid forever because HMAC verification does not depend on the
current time; only the freshness check does.

## Regenerating

For the synthetic payload, with the same token and `auth_date`, run outside the
repository (Python 3 standard library only):

```python
import base64, hashlib, hmac, json, os, urllib.parse

token = "7342037359:AAHI25ES9xCOMPokpYoz-p8XVrZUdygo2J4"
user = json.dumps({"id": 424242, "first_name": "A&B=C", "last_name": "Doe",
                   "username": "amp_user", "language_code": "en"},
                  separators=(",", ":"))
fields = {
    "query_id": "AAHdF6IQAAAAAN0XohDhrOrc",
    "user": user,
    "auth_date": "1733509682",
    "start_param": "chat_-1001234567890",
    "signature": base64.urlsafe_b64encode(os.urandom(64)).rstrip(b"=").decode(),
}
check = "\n".join(f"{k}={fields[k]}" for k in sorted(fields))
secret = hmac.new(b"WebAppData", token.encode(), hashlib.sha256).digest()
fields["hash"] = hmac.new(secret, check.encode(), hashlib.sha256).hexdigest()
print(urllib.parse.urlencode(fields))
```

To recapture a real payload, open the deployed panel inside a Telegram client
from the test environment and copy the `tgWebAppData` value from the launch URL
(the `#tgWebAppData=` fragment); do not add it to the repository if it belongs
to a real user.
