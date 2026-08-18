# F613GV9 FactoryMode + verified temporary Telnet client

Purpose-built for the tested target:

```text
ZXHN F613GV9
Hardware V9.0
Software V2.2.0P1T11
HTTPS webFac on 192.168.1.1:443
re_rand handshake
```

The client performs the complete HTTPS FactoryMode flow, derives the AES-192
session key, builds the real-client-MAC proof, accepts only the verified
F613GV9 12-byte compatibility suffix, obtains temporary credentials, and then
proves them with a real Telnet shell login.

It deliberately does not make Telnet permanent or change the ONT database.

```powershell
go build -buildvcs=false -trimpath -o f613gv9_factory_telnet.exe .

.\f613gv9_factory_telnet.exe `
  -mac 'AA-BB-CC-DD-EE-FF' `
  -username 'CMCCAdmin' `
  -password '<YOUR_FACTORY_WEB_PASSWORD>'
```

Run only while the computer is connected directly to the authorized ONT and
the fibre is disconnected. This implementation is offline/unit-tested against
the public protocol and the observed F613GV9 response shape; final live-device
validation is intentionally pending the next controlled fibre-disconnected
maintenance window.

Protocol acknowledgements:

- https://gist.github.com/ovn-is/d0331f781f5468dfaf107765fe095d85
- https://github.com/Septrum101/zteOnu
- https://github.com/douniwan5788/zte_modem_tools/issues/20
