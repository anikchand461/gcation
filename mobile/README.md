# G-CATION Android app

A native Android app for G-CATION, written in Go with [Gio](https://gioui.org). It talks to the
same FastAPI backend as the website, so students and the admin see the same data everywhere.

## What's inside

**Students:** register and log in, browse courses, pay by UPI QR (primary and secondary, or open a
UPI app directly), tell the team they've paid, see access periods, read updates and join links,
download PDF receipts, change their password, and switch between light and dark themes.

**Admin** (choose *Admin* on the login screen):
- **Overview:** totals, payments to verify, access ending soon and recent payments.
- **Students:** search and filters. Each student has a detail page where you can call,
  WhatsApp or email them, give course access for a period, revoke access, download receipts,
  edit their details and reset their password.
- **Payments:** waiting claims and the payment history.
- **Updates:** post, edit and delete updates for paid students.
- **More:** the support phone, QR codes (upload from the phone), courses (add, edit, hide and
  delete) and the admin username and password.

## Build

You need Go 1.26+, `gogio` (`go install gioui.org/cmd/gogio@latest`), the Android SDK with the
NDK, and Java 17+.

```bash
./build.sh                            # build/gcation.apk using the live backend
API=http://10.0.2.2:8000 ./build.sh   # emulator, with the backend running on your computer
adb install -r build/gcation.apk
```

The backend address is set at build time with `API`. When running on a computer
(`go run .`), it can also come from the `GCATION_API` environment variable.

### Signing for the Play Store

`gogio` signs with a debug key by default. For the Play Store, create your own keystore once and
keep it safe, because every future update must be signed with the same key:

```bash
keytool -genkey -v -keystore gcation.keystore -alias gcation -keyalg RSA -keysize 2048 -validity 10000
SIGNKEY=gcation.keystore SIGNPASS=yourpassword ./build.sh
```

Never commit the keystore.

## Tests

```bash
go test ./internal/ui/                         # renders every screen headlessly
SHOTS_DIR=/tmp/shots go test ./internal/ui/    # also saves PNG screenshots
```

## Licences

The app uses the Inter font, which is licensed under the SIL Open Font License
(`internal/ui/assets/fonts/OFL.txt`).
