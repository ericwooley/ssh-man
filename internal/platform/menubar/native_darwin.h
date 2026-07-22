#ifndef SSH_MAN_MENUBAR_DARWIN_H
#define SSH_MAN_MENUBAR_DARWIN_H

typedef enum {
    SSHManBrowserSwitchForward = 1,
    SSHManBrowserSwitchBackward = 2,
} SSHManBrowserSwitchDirection;

int SSHManMenuBarShouldDismissPopupForOutsideClick(int applicationActive,
                                                    int popupVisible);
int SSHManMenuBarStart(void);
int SSHManMenuBarShow(void);
int SSHManMenuBarShowBrowserSwitcher(void);
void SSHManMenuBarCancelBrowserSwitchSession(void);
int SSHManMenuBarSetBrowserShortcuts(unsigned int forwardKeyCode,
                                     unsigned int forwardModifiers,
                                     unsigned int backwardKeyCode,
                                     unsigned int backwardModifiers);
void SSHManMenuBarStop(void);

#endif
