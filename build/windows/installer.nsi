; Windows installer for Agent Session Manager (asmgr).
;
; A command-line tool, so what this does beyond copying a file is:
;
;   - put the install directory on the user's PATH, since a CLI nobody can type
;     the name of is not installed in any useful sense;
;   - offer to install psmux, the multiplexer the app cannot work without. On
;     Linux and macOS tmux is a package-manager line away and usually already
;     there; psmux is a separate download most Windows users have never heard
;     of, so leaving them to find it is leaving them with an app that starts and
;     then does nothing.
;
; Per-user throughout: no elevation, installs under LOCALAPPDATA, writes only to
; HKCU. Asking for administrator to drop one executable somewhere is a poor
; trade, and winget installs this package without elevation too.

Unicode true

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "WordFunc.nsh"

; Passed in by the build: /DVERSION=0.8.0 /DARCH=amd64
!ifndef VERSION
  !define VERSION "0.0.0"
!endif
!ifndef ARCH
  !define ARCH "amd64"
!endif
!ifndef SOURCE_EXE
  !define SOURCE_EXE "asmgr.exe"
!endif

!define APP_NAME "Agent Session Manager"
!define APP_EXE "asmgr.exe"
!define PUBLISHER "izll"
!define WEBSITE "https://github.com/izll/agent-session-manager"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\asmgr"

Name "${APP_NAME}"
OutFile "../../dist/asmgr_${VERSION}_windows_${ARCH}_setup.exe"
InstallDir "$LOCALAPPDATA\Programs\asmgr"
InstallDirRegKey HKCU "Software\asmgr" "InstallDir"
RequestExecutionLevel user
SetCompressor /SOLID lzma

VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "${APP_NAME}"
VIAddVersionKey "FileDescription" "Terminal session manager for AI coding agents"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "CompanyName" "${PUBLISHER}"
VIAddVersionKey "LegalCopyright" "MIT"

!define MUI_ICON "icon.ico"
!define MUI_UNICON "icon.ico"
!define MUI_ABORTWARNING

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "../../LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
Page custom MultiplexerPage MultiplexerPageLeave
!insertmacro MUI_PAGE_INSTFILES

!define MUI_FINISHPAGE_TEXT "asmgr is installed and on your PATH.$\r$\n$\r$\nOpen a new terminal and run:$\r$\n    asmgr$\r$\n$\r$\nA terminal already open will not see the new PATH."
!define MUI_FINISHPAGE_LINK "Documentation and releases"
!define MUI_FINISHPAGE_LINK_LOCATION "${WEBSITE}"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Var Dialog
Var InstallPsmuxCheckbox
Var InstallPsmux
Var PsmuxStatusLabel

; Whether psmux can already be found. Checked with `where`, which searches PATH
; exactly as the app will.
Function PsmuxPresent
  nsExec::ExecToStack 'cmd /c where psmux'
  Pop $0
  ${If} $0 == 0
    Push "yes"
  ${Else}
    Push "no"
  ${EndIf}
FunctionEnd

Function WingetPresent
  nsExec::ExecToStack 'cmd /c where winget'
  Pop $0
  ${If} $0 == 0
    Push "yes"
  ${Else}
    Push "no"
  ${EndIf}
FunctionEnd

; A page about the multiplexer, shown only when there is something to say.
;
; Skipped entirely when psmux is already there — a page whose only content is
; "nothing to do here" is a page worth not showing.
Function MultiplexerPage
  ; Cleared before the page can be skipped: MultiplexerPageLeave does not run
  ; for a skipped page, so a stale or uninitialised value would decide whether
  ; winget is invoked.
  StrCpy $InstallPsmux "no"
  StrCpy $InstallPsmuxCheckbox 0

  Call PsmuxPresent
  Pop $R0
  ${If} $R0 == "yes"
    DetailPrint "psmux is already installed."
    Abort
  ${EndIf}

  !insertmacro MUI_HEADER_TEXT "Terminal multiplexer" \
    "asmgr runs every session inside psmux, and cannot work without it."

  nsDialogs::Create 1018
  Pop $Dialog
  ${If} $Dialog == error
    Abort
  ${EndIf}

  ${NSD_CreateLabel} 0 0 100% 40u \
    "Every session asmgr creates lives in a terminal multiplexer, which keeps \
your agents running when you close the window. On Windows that is psmux, a \
native multiplexer that speaks tmux's command language — no WSL or MSYS2 \
needed.$\r$\n$\r$\npsmux was not found on this machine."
  Pop $0

  Call WingetPresent
  Pop $R1
  ${If} $R1 == "yes"
    ${NSD_CreateCheckbox} 0 50u 100% 12u "Install psmux now (winget install marlocarlo.psmux)"
    Pop $InstallPsmuxCheckbox
    ${NSD_Check} $InstallPsmuxCheckbox
    ${NSD_CreateLabel} 0 66u 100% 20u \
      "This runs winget as you, without asking for administrator. You can \
untick it and install psmux yourself later."
    Pop $PsmuxStatusLabel
  ${Else}
    ; No winget: say where to get it rather than offering something that
    ; cannot run.
    ${NSD_CreateLabel} 0 50u 100% 30u \
      "winget is not available on this machine, so asmgr cannot install psmux \
for you. Install it from https://github.com/psmux/psmux/releases and put it on \
your PATH, then run asmgr."
    Pop $PsmuxStatusLabel
    StrCpy $InstallPsmux "no"
  ${EndIf}

  nsDialogs::Show
FunctionEnd

Function MultiplexerPageLeave
  ${If} $InstallPsmuxCheckbox != 0
    ${NSD_GetState} $InstallPsmuxCheckbox $0
    ${If} $0 == ${BST_CHECKED}
      StrCpy $InstallPsmux "yes"
    ${Else}
      StrCpy $InstallPsmux "no"
    ${EndIf}
  ${EndIf}
FunctionEnd

Section "asmgr" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"
  File "/oname=${APP_EXE}" "${SOURCE_EXE}"

  WriteRegStr HKCU "Software\asmgr" "InstallDir" "$INSTDIR"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Add/Remove Programs
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "${APP_NAME}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "${PUBLISHER}"
  WriteRegStr HKCU "${UNINST_KEY}" "URLInfoAbout" "${WEBSITE}"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${APP_EXE}"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1

  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKCU "${UNINST_KEY}" "EstimatedSize" "$0"

  Call AddToPath
SectionEnd

Section "-psmux" SecPsmux
  ; A silent install (/S) shows no pages, so MultiplexerPage never runs and
  ; never sets this — measured: asmgr installed, psmux did not, and the app
  ; then refused to start. An unattended install that leaves the thing unable
  ; to run is worse than one that takes a little longer, so the decision is
  ; made here for that case: install it if it is missing.
  ;
  ; Anyone who genuinely wants asmgr alone can pass /NOPSMUX.
  ${If} ${Silent}
    ${GetParameters} $R9
    ${GetOptions} $R9 "/NOPSMUX" $R8
    ${If} ${Errors}
      ClearErrors
      Call PsmuxPresent
      Pop $R7
      ${If} $R7 == "no"
        StrCpy $InstallPsmux "yes"
      ${EndIf}
    ${Else}
      DetailPrint "/NOPSMUX given; not installing psmux."
    ${EndIf}
  ${EndIf}

  ${If} $InstallPsmux == "yes"
    Call WingetPresent
    Pop $R6
    ${If} $R6 == "no"
      DetailPrint "winget is not available; cannot install psmux."
      DetailPrint "Get it from https://github.com/psmux/psmux/releases"
      Return
    ${EndIf}

    DetailPrint "Installing psmux with winget..."
    ; --silent so the install does not stop on a prompt behind the installer
    ; window; --accept-*-agreements because an unattended run cannot answer
    ; them and would otherwise hang.
    nsExec::ExecToLog 'cmd /c winget install --id marlocarlo.psmux --silent \
--accept-package-agreements --accept-source-agreements'
    Pop $0
    ${If} $0 == 0
      DetailPrint "psmux installed."
    ${Else}
      ; Not fatal: asmgr is installed either way, and it says what is missing
      ; when it starts. Failing the whole installation over an optional
      ; dependency would be worse than reporting it.
      DetailPrint "winget could not install psmux (exit code $0)."
      DetailPrint "Install it yourself with: winget install --id marlocarlo.psmux"
    ${EndIf}
  ${EndIf}
SectionEnd

; PATH, per user.
;
; Read from the registry rather than $%PATH%, which is this process's copy and
; may already be truncated to 1024 characters. Written back with the expandable
; type so entries like %USERPROFILE% in an existing PATH keep working.
;
; The membership test uses WordFind rather than a hand-rolled substring search:
; PATH is a ;-separated list, and a plain "does it contain" match would treat
; C:\...\asmgr-old as C:\...\asmgr already being there.
Function AddToPath
  ReadRegStr $0 HKCU "Environment" "PATH"

  ${If} $0 == ""
    WriteRegExpandStr HKCU "Environment" "PATH" "$INSTDIR"
    DetailPrint "Added $INSTDIR to PATH."
    Goto notify
  ${EndIf}

  ; Walk the entries and compare each one whole.
  StrCpy $1 1
  entryLoop:
    ${WordFind} "$0" ";" "E+$1" $2
    IfErrors pathDone
    ${If} $2 == "$INSTDIR"
      DetailPrint "Already on PATH."
      Return
    ${EndIf}
    IntOp $1 $1 + 1
    Goto entryLoop
  pathDone:
  ClearErrors

  WriteRegExpandStr HKCU "Environment" "PATH" "$0;$INSTDIR"
  DetailPrint "Added $INSTDIR to PATH."

  notify:
  ; Tell running processes, so a terminal opened afterwards sees it without a
  ; sign-out. Terminals already open keep their copy — hence the note on the
  ; finish page.
  SendMessage ${HWND_BROADCAST} ${WM_WININICHANGE} 0 "STR:Environment" /TIMEOUT=5000
FunctionEnd

Function un.RemoveFromPath
  ReadRegStr $0 HKCU "Environment" "PATH"
  ${If} $0 == ""
    Return
  ${EndIf}

  ; Rebuilt entry by entry rather than string-replaced, so a PATH containing
  ; this directory as part of a longer entry is left intact.
  StrCpy $1 ""
  StrCpy $2 1
  entryLoop:
    ${WordFind} "$0" ";" "E+$2" $3
    IfErrors pathDone
    ${If} $3 != ""
    ${AndIf} $3 != "$INSTDIR"
      ${If} $1 == ""
        StrCpy $1 "$3"
      ${Else}
        StrCpy $1 "$1;$3"
      ${EndIf}
    ${EndIf}
    IntOp $2 $2 + 1
    Goto entryLoop
  pathDone:
  ClearErrors

  WriteRegExpandStr HKCU "Environment" "PATH" "$1"
  SendMessage ${HWND_BROADCAST} ${WM_WININICHANGE} 0 "STR:Environment" /TIMEOUT=5000
FunctionEnd

Section "Uninstall"
  Call un.RemoveFromPath

  Delete "$INSTDIR\${APP_EXE}"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  DeleteRegKey HKCU "${UNINST_KEY}"
  DeleteRegKey HKCU "Software\asmgr"

  ; Sessions and settings are left alone: they are the user's, an uninstall is
  ; often an upgrade, and psmux may have been installed for other reasons.
SectionEnd
