; Inno Setup script for DofHunt
; Compile with Inno Setup Compiler (iscc)

#define MyAppName "DofHunt"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "DofHunt"
#define MyAppExeName "dofhunt.exe"

[Setup]
AppId={{0EAB12B5-4C7A-4B8C-9D2D-B12EF8A9A9AA}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=..\..\build
OutputBaseFilename=dofhunt-installer-windows
Compression=lzma
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "..\..\build\release-windows\dofhunt.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\build\release-windows\clues.json"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\build\release-windows\.env"; DestDir: "{app}"; Flags: onlyifdoesntexist skipifsourcedoesntexist
Source: "..\..\build\release-windows\.env.example"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist
Source: "..\..\build\release-windows\install-windows\*"; DestDir: "{app}\install-windows"; Flags: recursesubdirs createallsubdirs ignoreversion

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop icon"; GroupDescription: "Additional icons:"; Flags: unchecked

[Run]
Filename: "{app}\install-windows\install-deps.bat"; Description: "Install OCR dependencies (required)"; Flags: postinstall shellexec waituntilterminated
Filename: "{app}\{#MyAppExeName}"; Description: "Launch {#MyAppName}"; Flags: postinstall nowait skipifsilent
