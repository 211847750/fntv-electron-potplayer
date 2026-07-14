!macro customInstall
  IfFileExists "$PROGRAMFILES32\WinFsp\bin\winfsp-x64.dll" winfsp_done 0
  IfFileExists "$PROGRAMFILES64\WinFsp\bin\winfsp-x64.dll" winfsp_done 0

  IfFileExists "$INSTDIR\third_party\winfsp\winfsp-2.1.25156.msi" 0 winfsp_missing
  DetailPrint "Installing WinFsp runtime..."
  ExecWait '"$SYSDIR\msiexec.exe" /i "$INSTDIR\third_party\winfsp\winfsp-2.1.25156.msi" /qn /norestart' $0
  IntCmp $0 0 winfsp_done 0 0
  IntCmp $0 3010 winfsp_done 0 0
  IntCmp $0 1641 winfsp_done 0 0

  MessageBox MB_ICONSTOP|MB_OK "WinFsp installation failed with exit code $0. FNMedia PotPlayer needs WinFsp for the virtual drive."
  Abort

winfsp_missing:
  MessageBox MB_ICONSTOP|MB_OK "WinFsp installer is missing from the package. Please rebuild the installer."
  Abort

winfsp_done:
!macroend
