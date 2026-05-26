# potplayer-bridge

Windows-only PotPlayer control bridge for the fntv Electron client.

The bridge launches PotPlayer, binds to the visible PotPlayer window, reads
playback state through PotPlayer's Win32 message interface, and writes JSON
Lines events to stdout.

## Build

```bash
GOOS=windows GOARCH=amd64 go build -o ../../../third_party/potplayer-bridge/potbridge.exe ./cmd/potbridge
```

## Usage

```powershell
.\potbridge.exe play `
  --potplayer "C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe" `
  --url "http://127.0.0.1:22345/api/v1/playvideo/xxx" `
  --title "Video title" `
  --seek 123 `
  --sub "C:\temp\subtitle.srt"
```

Each stdout line is a JSON event:

```json
{"type":"ready","hwnd":41225930}
{"type":"progress","posMs":1109,"durMs":756778,"status":1}
{"type":"exit","posMs":120000,"durMs":756778,"status":-1}
```

The Win32 state query mirrors the verified PowerShell flow:

- `EnumWindows`
- `IsWindowVisible`
- `GetClassName`
- match `PotPlayer64` or `PotPlayer`
- `SendMessage(hwnd, WM_USER, 0x5004/0x5002/0x5006, 0)`
