; Headliner's Windows installer (ONLINE.md §A5.4), built by
; .github/workflows/release.yml:  ISCC /DAppVersion=1.2.0 installer/headliner.iss
;
; Per-user install (no admin) into %LOCALAPPDATA%\Programs\Headliner, so the
; updater can replace files without a UAC prompt. Uninstalling never
; deletes saves (%APPDATA%\headliner).

#ifndef AppVersion
  #define AppVersion "0.0.0"
#endif

[Setup]
AppId={{8E2B6C1F-6A3D-4C55-9E57-4B0B7D2A1C11}
AppName=Headliner
AppVersion={#AppVersion}
AppVerName=Headliner {#AppVersion}
AppPublisher=Headliner
DefaultDirName={localappdata}\Programs\Headliner
DefaultGroupName=Headliner
DisableProgramGroupPage=yes
DisableDirPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir=..\dist
OutputBaseFilename=Headliner-Setup-{#AppVersion}
Compression=lzma2
SolidCompression=yes
UninstallDisplayIcon={app}\headliner.exe
UninstallDisplayName=Headliner
LicenseFile=..\LICENSE
CloseApplications=yes

[Tasks]
Name: "desktopicon"; Description: "Put a Headliner shortcut on the desktop"; GroupDescription: "Shortcuts:"; Flags: unchecked

[Files]
Source: "..\dist\headliner.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\NOTICE.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\Headliner"; Filename: "{app}\headliner.exe"
Name: "{userdesktop}\Headliner"; Filename: "{app}\headliner.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\headliner.exe"; Description: "Start Headliner"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
; The updater's previous version and any half-made update; never the saves.
Type: filesandordirs; Name: "{app}.prev"
Type: filesandordirs; Name: "{app}.new"
Type: filesandordirs; Name: "{app}.bad"

[Code]
function InitializeUninstall(): Boolean;
begin
  Result := MsgBox('This removes the Headliner program.' + #13#10 + #13#10 +
    'Your progress, settings and songs are NOT deleted. They stay in' + #13#10 +
    ExpandConstant('{userappdata}\headliner') + #13#10 + #13#10 +
    'Uninstall Headliner?', mbConfirmation, MB_YESNO) = IDYES;
end;
