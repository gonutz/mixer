#include "dsound_wrapper.h"

static LPDIRECTSOUND DirectSoundObject = 0;
static LPDIRECTSOUNDBUFFER PrimarySoundBuffer = 0;
static LPDIRECTSOUNDBUFFER GlobalSoundBuffer = 0;

typedef HRESULT WINAPI direct_sound_create(
	LPCGUID DeviceGuid,
	LPDIRECTSOUND* ppDS,
	LPUNKNOWN outer);

HRESULT initDirectSound(DWORD samplesPerSecond, int* errorContext) {
	*errorContext = NoError;
	HRESULT result = DS_OK;

	HMODULE directSoundLib = LoadLibraryA("dsound.dll");
	if (!directSoundLib) {
		*errorContext = LoadLibraryFailed;
		return DSERR_GENERIC;
	}

	direct_sound_create* DirectSoundCreate = (direct_sound_create*)
		GetProcAddress(directSoundLib, "DirectSoundCreate");
	if (!DirectSoundCreate) {
		*errorContext = DirectSoundCreateMissing;
		return DSERR_GENERIC;
	}

	LPDIRECTSOUND directSound;
	result = DirectSoundCreate(0, &directSound, 0);
	if (!SUCCEEDED(result)) {
		*errorContext = DirectSoundCreateFailed;
		return result;
	}
	DirectSoundObject = directSound;

	// TODO second parameter should be the window handle
	result = directSound->lpVtbl->SetCooperativeLevel(
		directSound,
		GetDesktopWindow(),
		DSSCL_PRIORITY
	);
	if (!SUCCEEDED(result)) {
		*errorContext = SetCooperativeLevelFailed;
		return result;
	}

	LPDIRECTSOUNDBUFFER primaryBuffer;
	DSBUFFERDESC primaryBufferDescription = {0};
	primaryBufferDescription.dwSize = sizeof(DSBUFFERDESC);
	primaryBufferDescription.dwFlags = DSBCAPS_PRIMARYBUFFER;
	primaryBufferDescription.dwBufferBytes = 0; // NOTE must be 0 for primary buffer
	result = directSound->lpVtbl->CreateSoundBuffer(
		directSound,
		&primaryBufferDescription,
		&primaryBuffer,
		0
	);
	if (!SUCCEEDED(result)) {
		*errorContext = CreatePrimarySoundBufferFailed;
		return result;
	}
	PrimarySoundBuffer = primaryBuffer;

	WAVEFORMATEX waveFormat = {0};
	waveFormat.wFormatTag      = WAVE_FORMAT_PCM;
	waveFormat.nChannels       = 2;
	waveFormat.nSamplesPerSec  = samplesPerSecond;
	waveFormat.wBitsPerSample  = 16; // NOTE must be 8 or 16
	waveFormat.nBlockAlign     = (waveFormat.nChannels*waveFormat.wBitsPerSample) / 8;
	waveFormat.nAvgBytesPerSec = waveFormat.nSamplesPerSec*waveFormat.nBlockAlign;
	result = primaryBuffer->lpVtbl->SetFormat(primaryBuffer, &waveFormat);
	if (!SUCCEEDED(result)) {
		*errorContext = PrimarySetFormatFailed;
		return result;
	}

	LPDIRECTSOUNDBUFFER secondaryBuffer;
	DSBUFFERDESC secondaryBufferDescription = {0};
	secondaryBufferDescription.dwSize = sizeof(DSBUFFERDESC);
	// TODO what should the flags really be? Really have DSBCAPS_GLOBALFOCUS?
	// Or have that as a flag to the init function?
	secondaryBufferDescription.dwFlags =
		DSBCAPS_GETCURRENTPOSITION2 | DSBCAPS_GLOBALFOCUS; 
	// TODO make this a parameter, for now it is 2 seconds
	// -> maybe have a go function take a time as paramter for specifying how
	// long the buffer should be and then compute the respective byte size and
	// pass it in to this function
	secondaryBufferDescription.dwBufferBytes = 2*waveFormat.nAvgBytesPerSec;
	secondaryBufferDescription.lpwfxFormat = &waveFormat;
	result = directSound->lpVtbl->CreateSoundBuffer(
		directSound,
		&secondaryBufferDescription,
		&secondaryBuffer,
		0
	);
	if (!SUCCEEDED(result)) {
		*errorContext = CreateSecondarySoundBufferFailed;
		return result;
	}

	GlobalSoundBuffer = secondaryBuffer;

	*errorContext = NoError;
	return DS_OK;
}

#define ReleaseObject(obj) if (obj) {obj->lpVtbl->Release(obj);	obj = 0;}

void closeDirectSound() {
	ReleaseObject(GlobalSoundBuffer);
	ReleaseObject(PrimarySoundBuffer);
	ReleaseObject(DirectSoundObject);
}

HRESULT startSound(int* errorContext) {
	*errorContext = NoError;
	
	if (!GlobalSoundBuffer) {
		*errorContext = GlobalSoundBufferNotSet;
		return DSERR_GENERIC;
	}
	
	HRESULT result;
	result = GlobalSoundBuffer->lpVtbl->Play(GlobalSoundBuffer, 0, 0, DSBPLAY_LOOPING);
	if (!SUCCEEDED(result)) {
		*errorContext = PlayingSoundBufferFailed;
		return result;
	}
	
	return DS_OK;
}

HRESULT stopSound(int* errorContext) {
	*errorContext = NoError;
	
	if (!GlobalSoundBuffer) {
		*errorContext = GlobalSoundBufferNotSet;
		return DSERR_GENERIC;
	}
	
	HRESULT result;
	result = GlobalSoundBuffer->lpVtbl->Stop(GlobalSoundBuffer);
	if (!SUCCEEDED(result)) {
		*errorContext = PlayingSoundBufferFailed;
		return result;
	}
	
	return DS_OK;
}

HRESULT getPlayAndWriteCursors(DWORD* playCursor, DWORD* writeCursor, int* errorContext) {
	*errorContext = NoError;
	
	if (!GlobalSoundBuffer) {
		*errorContext = GlobalSoundBufferNotSet;
		return DSERR_GENERIC;
	}

	HRESULT result;
	result = GlobalSoundBuffer->lpVtbl->GetCurrentPosition(
		GlobalSoundBuffer,
		playCursor,
		writeCursor
	);
	if (!SUCCEEDED(result)) {
		*errorContext = GetCurrentPositionFailed;
		return result;
	}
	
	return DS_OK;
}

HRESULT copyToSoundBuffer(DWORD offset, char* data, DWORD byteCount, int* errorContext) {
	*errorContext = NoError;
	HRESULT result;
	
	if (!GlobalSoundBuffer) {
		*errorContext = GlobalSoundBufferNotSet;
		return DSERR_GENERIC;
	}

	VOID* region1;
	DWORD region1Size;
	VOID* region2;
	DWORD region2Size;
	result = GlobalSoundBuffer->lpVtbl->Lock(
		GlobalSoundBuffer,
		offset,
		byteCount,
		&region1,
		&region1Size,
		&region2,
		&region2Size,
		0
	);
	if (!SUCCEEDED(result)) {
		*errorContext = LockFailed;
		return result;
	}
	
	char* output = (char*)region1;
	int i;
	for (i = 0; i < region1Size; ++i) {
		*output++ = *data++;
	}
	
	output = (char*)region2;
	for (i = 0; i < region2Size; ++i) {
		*output++ = *data++;
	}

	result = GlobalSoundBuffer->lpVtbl->Unlock(
		GlobalSoundBuffer,
		region1,
		region1Size,
		region2,
		region2Size
	);
	if (!SUCCEEDED(result)) {
		*errorContext = UnlockFailed;
		return result;
	}

	return DS_OK;
}
