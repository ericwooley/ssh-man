Unicode true

!ifndef SSH_MAN_CHANNEL
  !define SSH_MAN_CHANNEL "stable"
!endif
!ifndef SSH_MAN_VERSION
  !define SSH_MAN_VERSION "1.0.0"
!endif

!define STABLE_PRODUCT_CODE "tech.moonpixels.ssh-man"
!define EXPERIMENTAL_PRODUCT_CODE "tech.moonpixels.ssh-man.experimental"

!if "${SSH_MAN_CHANNEL}" == "stable"
  !define CHANNEL_PRODUCT_NAME "SSH Man"
  !define CHANNEL_PRODUCT_CODE "${STABLE_PRODUCT_CODE}"
  !define CHANNEL_INSTALLER_FILENAME "ssh-man-windows-amd64-installer.exe"
!else
  !if "${SSH_MAN_CHANNEL}" == "experimental"
    !define CHANNEL_PRODUCT_NAME "SSH Man Experimental"
    !define CHANNEL_PRODUCT_CODE "${EXPERIMENTAL_PRODUCT_CODE}"
    !define CHANNEL_INSTALLER_FILENAME "ssh-man-experimental-windows-amd64-installer.exe"
  !else
    !error "SSH_MAN_CHANNEL must be stable or experimental"
  !endif
!endif

!define INFO_PROJECTNAME "ssh-man"
!define INFO_COMPANYNAME "Eric Wooley"
!define INFO_PRODUCTNAME "${CHANNEL_PRODUCT_NAME}"
!define INFO_PRODUCTVERSION "${SSH_MAN_VERSION}"
!define INFO_COPYRIGHT "Copyright Eric Wooley"
!define PRODUCT_EXECUTABLE "ssh-man.exe"
!define UNINST_KEY_NAME "${CHANNEL_PRODUCT_CODE}"

!include "wails_tools.nsh"

VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName" "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright" "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName" "${INFO_PRODUCTNAME}"

ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_FINISHPAGE_NOAUTOCLOSE
!define MUI_ABORTWARNING

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\bin\${CHANNEL_INSTALLER_FILENAME}"
InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
ShowInstDetails show

Function .onInit
  !insertmacro wails.checkArchitecture
FunctionEnd

Section
  !insertmacro wails.setShellContext
  !insertmacro wails.webview2runtime

  SetOutPath $INSTDIR

  Delete "$INSTDIR\${PRODUCT_EXECUTABLE}.new"
  ClearErrors
  File "/oname=${PRODUCT_EXECUTABLE}.new" "${ARG_WAILS_AMD64_BINARY}"
  IfErrors binaryInstallFailed

  IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}.old" 0 binaryCheckExisting
  ClearErrors
  Delete "$INSTDIR\${PRODUCT_EXECUTABLE}.old"
  IfErrors binaryInstallFailed

binaryCheckExisting:
  IfFileExists "$INSTDIR\${PRODUCT_EXECUTABLE}" 0 binaryPromote
  ClearErrors
  Rename "$INSTDIR\${PRODUCT_EXECUTABLE}" "$INSTDIR\${PRODUCT_EXECUTABLE}.old"
  IfErrors binaryInstallFailed

binaryPromote:
  ClearErrors
  Rename "$INSTDIR\${PRODUCT_EXECUTABLE}.new" "$INSTDIR\${PRODUCT_EXECUTABLE}"
  IfErrors binaryRestoreExisting
  Delete /REBOOTOK "$INSTDIR\${PRODUCT_EXECUTABLE}.old"
  Goto binaryInstalled

binaryRestoreExisting:
  Rename "$INSTDIR\${PRODUCT_EXECUTABLE}.old" "$INSTDIR\${PRODUCT_EXECUTABLE}"

binaryInstallFailed:
  Delete "$INSTDIR\${PRODUCT_EXECUTABLE}.new"
  IfSilent binarySetFailure binaryShowFailure

binaryShowFailure:
  MessageBox MB_ICONSTOP|MB_OK \
    "SSH Man could not be updated. Close SSH Man and run the installer again."

binarySetFailure:
  SetErrorLevel 1
  Abort

binaryInstalled:
  CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
  CreateShortcut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

  !insertmacro wails.associateFiles
  !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall"
  !insertmacro wails.setShellContext

  RMDir /r "$AppData\${PRODUCT_EXECUTABLE}"
  RMDir /r $INSTDIR

  Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
  Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

  !insertmacro wails.unassociateFiles
  !insertmacro wails.deleteUninstaller
SectionEnd
