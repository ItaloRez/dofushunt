![image](https://github.com/cjbrigato/dofhunt/blob/main/winres/logo.png?raw=true)

A quick app to ease with dofus 3.0 treasure hunts without
depending on dofusdb closed datas

---

## Experience immediate and responsive treasure hunting

[dofhunt.webm](https://github.com/user-attachments/assets/0b323ced-9469-4675-82e9-8432f80b1db7)

## Multilang support

![image](https://github.com/user-attachments/assets/7743de2e-9be4-44c1-bff5-995b349d25b6)

## Install

- Download latest release https://github.com/cjbrigato/dofhunt/releases/latest
- Just run the executable, it is now all standalone

## One-click dependency setup (OCR)

For non-technical users, ship these installer scripts together with your app release.

### macOS

1. Double-click `install/macos/install.command`
2. Wait until the installer finishes
3. Open DofHunt normally

### Windows

1. Double-click `install/windows/install-deps.bat`
2. Accept UAC admin prompt
3. Wait until the installer finishes
4. Open DofHunt normally

These scripts install Tesseract/Leptonica prerequisites used by OCR.

## Windows

- https://github.com/cjbrigato/dofhunt/releases/download/alpha-0.10-ui-sugar/dofhunt-win64.exe

## Release checklist (recommended)

- Include executable + `clues.json`
- Include installer scripts under `install/`
- Zip everything so end user only needs double-click install, then run app

## Generate Windows wizard installer (.exe)

1. On a Windows machine, run `scripts/release-windows.ps1`
2. Install Inno Setup: https://jrsoftware.org/isinfo.php
3. Open `installer/windows/dofhunt.iss` in Inno Setup Compiler
4. Click **Build**
5. The wizard installer will be generated in `build/dofhunt-installer-windows.exe`

### About .env in the installer

- If `.env` exists during release build, it is included in the package
- `.env.example` is also included as reference
- Installer uses `onlyifdoesntexist` for `.env` (keeps user-edited `.env` on app updates)

## building

- Depends on GIU https://github.com/AllenDang/giu so requirements are same
- https://github.com/AllenDang/giu?tab=readme-ov-file#windows
- then `go build -ldflags -H=windowsgui`

## old long demo

(a bit old)
[![DofHunt](https://img.youtube.com/vi/Pcuv9M-DRMM/0.jpg)](https://www.youtube.com/watch?v=Pcuv9M-DRMM)
