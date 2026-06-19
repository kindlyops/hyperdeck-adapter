; NSIS installer for HyperDeck Adapter.
; Build: makensis /DVERSION=<x.y.z> /DOUTFILE=<path> build/windows/installer.nsi
; Run from the repository root so relative paths resolve.

Unicode true
!include "MUI2.nsh"
!include "x64.nsh"

!ifndef VERSION
  !define VERSION "0.0.0"
!endif
!ifndef SRCDIR
  !define SRCDIR "build\win"
!endif
!ifndef OUTFILE
  !define OUTFILE "hyperdeck-adapter-setup.exe"
!endif

Name "HyperDeck Adapter ${VERSION}"
OutFile "${OUTFILE}"
InstallDir "$PROGRAMFILES64\HyperDeck Adapter"
InstallDirRegKey HKLM "Software\HyperDeck Adapter" "InstallDir"
RequestExecutionLevel admin
SetCompressor /SOLID lzma

VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "HyperDeck Adapter"
VIAddVersionKey "FileDescription" "HyperDeck Adapter installer"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "CompanyName" "Kindly Ops, LLC"
VIAddVersionKey "LegalCopyright" "Copyright 2026 Kindly Ops, LLC"

!define MUI_ICON "build\icon\app.ico"
!define MUI_UNICON "build\icon\app.ico"
!define MUI_ABORTWARNING

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\HyperDeckAdapter"

Section "HyperDeck Adapter" SecMain
  SetOutPath "$INSTDIR"
  File "${SRCDIR}\hyperdeck-adapter.exe"
  File "${SRCDIR}\injcheck.exe"
  File "${SRCDIR}\README.md"
  File "${SRCDIR}\LICENSE"
  SetOutPath "$INSTDIR\examples"
  File /r "${SRCDIR}\examples\*.*"

  CreateDirectory "$SMPROGRAMS\HyperDeck Adapter"
  CreateShortcut "$SMPROGRAMS\HyperDeck Adapter\HyperDeck Adapter.lnk" "$INSTDIR\hyperdeck-adapter.exe"
  CreateShortcut "$SMPROGRAMS\HyperDeck Adapter\Uninstall.lnk" "$INSTDIR\uninstall.exe"

  WriteRegStr HKLM "Software\HyperDeck Adapter" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayName" "HyperDeck Adapter"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNINST_KEY}" "Publisher" "Kindly Ops, LLC"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\hyperdeck-adapter.exe"
  WriteRegStr HKLM "${UNINST_KEY}" "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoRepair" 1
  WriteUninstaller "$INSTDIR\uninstall.exe"
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\hyperdeck-adapter.exe"
  Delete "$INSTDIR\injcheck.exe"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\LICENSE"
  RMDir /r "$INSTDIR\examples"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\HyperDeck Adapter\HyperDeck Adapter.lnk"
  Delete "$SMPROGRAMS\HyperDeck Adapter\Uninstall.lnk"
  RMDir "$SMPROGRAMS\HyperDeck Adapter"
  DeleteRegKey HKLM "${UNINST_KEY}"
  DeleteRegKey HKLM "Software\HyperDeck Adapter"
SectionEnd
