#include <windows.h>
#include <dsound.h>

#define NoError                          0
#define LoadLibraryFailed                1
#define DirectSoundCreateMissing         2
#define DirectSoundCreateFailed          3
#define SetCooperativeLevelFailed        4
#define CreatePrimarySoundBufferFailed   5
#define PrimarySetFormatFailed           6
#define CreateSecondarySoundBufferFailed 7
#define PlayingSoundBufferFailed         8

#define GlobalSoundBufferNotSet   9
#define GetCurrentPositionFailed 10
#define LockFailed               11
#define UnlockFailed             12

// initDirectSound initializes DirectSound and creates a 2 channel 16 bit sound
// buffer with a length to hold 2 seconds of audio data.
// The associated window handle is that of the desktop, this means that the
// played sound is always audible and does not stop when the window focus
// changes.
HRESULT initDirectSound(DWORD samplesPerSecond, int* errorContext);
void closeDirectSound();

HRESULT startSound(int* errorContext);
HRESULT stopSound(int* errorContext);

HRESULT getPlayAndWriteCursors(DWORD* playCursor, DWORD* writeCursor, int* errorContext);
HRESULT copyToSoundBuffer(DWORD offset, char* data, DWORD byteCount, int* errorContext);