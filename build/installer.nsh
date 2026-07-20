!ifndef BUILD_UNINSTALLER
Function checkWinFspInstalled
  StrCpy $3 "0"
  IfFileExists "$PROGRAMFILES32\WinFsp\bin\winfsp-x64.dll" 0 +3
  StrCpy $3 "1"
  Return
  IfFileExists "$PROGRAMFILES64\WinFsp\bin\winfsp-x64.dll" 0 +3
  StrCpy $3 "1"
  Return

  SetRegView 32
  ReadRegStr $1 HKLM "SOFTWARE\WinFsp" "InstallDir"
  StrCmp $1 "" +4 0
  IfFileExists "$1\bin\winfsp-x64.dll" 0 +3
  StrCpy $3 "1"
  Return

  ExecWait '"$SYSDIR\sc.exe" query WinFsp.Launcher' $2
  IntCmp $2 0 0 +3 +3
  StrCpy $3 "1"
  Return
FunctionEnd

!macro customInstall
  Call checkWinFspInstalled
  StrCmp $3 "1" winfsp_done 0

  IfFileExists "$INSTDIR\third_party\winfsp\winfsp-2.1.25156.msi" 0 winfsp_missing
  DetailPrint "Installing WinFsp runtime..."
  StrCpy $4 "$TEMP\FNMedia-PotPlayer-WinFsp-install.log"
  Delete "$4"
  ExecWait '"$SYSDIR\msiexec.exe" /i "$INSTDIR\third_party\winfsp\winfsp-2.1.25156.msi" /qn /norestart /L*V "$4"' $0
  IntCmp $0 0 winfsp_done 0 0
  IntCmp $0 3010 winfsp_done 0 0
  IntCmp $0 1641 winfsp_done 0 0
  Call checkWinFspInstalled
  StrCmp $3 "1" winfsp_done 0

  MessageBox MB_ICONSTOP|MB_OK "WinFsp installation failed with exit code $0. FNMedia PotPlayer needs WinFsp for the virtual drive.$\r$\n$\r$\nLog: $4"
  Abort

winfsp_missing:
  MessageBox MB_ICONSTOP|MB_OK "WinFsp installer is missing from the package. Please rebuild the installer."
  Abort

winfsp_done:
!macroend
!endif
