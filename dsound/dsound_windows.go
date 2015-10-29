package dsound

/*
#cgo LDFLAGS: -lkernel32

#include "dsound_wrapper.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"strconv"
	"unsafe"
)

func Init(samplesPerSecond int) error {
	if samplesPerSecond <= 0 {
		return errors.New(
			"initDirectSound: illegal samplesPerSound: " +
				strconv.Itoa(samplesPerSecond))
	}
	var errContext C.int
	result := C.initDirectSound(C.DWORD(samplesPerSecond), &errContext)
	return makeError("initDirectSound", result, errContext)
}

func Close() {
	C.closeDirectSound()
}

func BufferSize() uint {
	return uint(C.getBufferSize())
}

func StartSound() error {
	var errContext C.int
	result := C.startSound(&errContext)
	return makeError("startSound", result, errContext)
}

func StopSound() error {
	var errContext C.int
	result := C.stopSound(&errContext)
	return makeError("stopSound", result, errContext)
}

func WriteToSoundBuffer(data []byte, offset uint) error {
	buffer := C.CString(string(data))
	defer C.free(unsafe.Pointer(buffer))
	var errContext C.int
	result := C.copyToSoundBuffer(C.DWORD(offset), buffer, C.DWORD(len(data)), &errContext)
	return makeError("writeToSoundBuffer", result, errContext)
}

func GetPlayAndWriteCursors() (play, write uint, err error) {
	var playCursor, writeCursor C.DWORD
	var errContext C.int
	result := C.getPlayAndWriteCursors(&playCursor, &writeCursor, &errContext)
	play, write = uint(playCursor), uint(writeCursor)
	err = makeError("getPlayAndWriteCursors", result, errContext)
	return
}

func surroundError(with string, err error) error {
	if err == nil {
		return nil
	}
	return errors.New(with + err.Error())
}

func makeError(funcName string, result C.HRESULT, context C.int) error {
	err := convertHRESULTtoError(result)
	if err == nil {
		return nil
	}
	return fmt.Errorf(
		"%s (%s): %s",
		funcName, contextDescription(context), err.Error(),
	)
}

func convertHRESULTtoError(result C.HRESULT) error {
	switch result {
	case C.DS_OK:
		return nil
	case C.DS_NO_VIRTUALIZATION:
		return errors.New("The buffer was created, but another 3D algorithm was substituted.")
	case C.DS_INCOMPLETE:
		return errors.New("The method succeeded, but not all the optional effects were obtained.")
	case C.DSERR_ACCESSDENIED:
		return errors.New("The request failed because access was denied.")
	case C.DSERR_ALLOCATED:
		return errors.New("The request failed because resources, such as a priority level, were already in use by another caller.")
	case C.DSERR_ALREADYINITIALIZED:
		return errors.New("The object is already initialized.")
	case C.DSERR_BADFORMAT:
		return errors.New("The specified wave format is not supported. ")
	case C.DSERR_BADSENDBUFFERGUID:
		return errors.New("The GUID specified in an audiopath file does not match a valid mix-in buffer.")
	case C.DSERR_BUFFERLOST:
		return errors.New("The buffer memory has been lost and must be restored.")
	case C.DSERR_BUFFERTOOSMALL:
		return errors.New("The buffer size is not great enough to enable effects processing.")
	case C.DSERR_CONTROLUNAVAIL:
		return errors.New("The buffer control (volume, pan, and so on) requested by the caller is not available. Controls must be specified when the buffer is created, using the dwFlags member of DSBUFFERDESC.")
	case C.DSERR_DS8_REQUIRED:
		return errors.New("A DirectSound object of class CLSID_DirectSound8 or later is required for the requested functionality. For more information, see IDirectSound8 Interface.")
	case C.DSERR_FXUNAVAILABLE:
		return errors.New("The effects requested could not be found on the system, or they are in the wrong order or in the wrong location; for example, an effect expected in hardware was found in software.")
	case C.DSERR_GENERIC:
		return errors.New("An undetermined error occurred inside the DirectSound subsystem.")
	case C.DSERR_INVALIDCALL:
		return errors.New("This function is not valid for the current state of this object.")
	case C.DSERR_INVALIDPARAM:
		return errors.New("An invalid parameter was passed to the returning function.")
	case C.DSERR_NOAGGREGATION:
		return errors.New("The object does not support aggregation.")
	case C.DSERR_NODRIVER:
		return errors.New("No sound driver is available for use, or the given GUID is not a valid DirectSound device ID.")
	case C.DSERR_NOINTERFACE:
		return errors.New("The requested COM interface is not available.")
	case C.DSERR_OBJECTNOTFOUND:
		return errors.New("The requested object was not found.")
	case C.DSERR_OTHERAPPHASPRIO:
		return errors.New("Another application has a higher priority level, preventing this call from succeeding.")
	case C.DSERR_OUTOFMEMORY:
		return errors.New("The DirectSound subsystem could not allocate sufficient memory to complete the caller's request.")
	case C.DSERR_PRIOLEVELNEEDED:
		return errors.New("A cooperative level of DSSCL_PRIORITY or higher is required.")
	case C.DSERR_SENDLOOP:
		return errors.New("A circular loop of send effects was detected.")
	case C.DSERR_UNINITIALIZED:
		return errors.New("The IDirectSound8::Initialize method has not been called or has not been called successfully before other methods were called.")
	case C.DSERR_UNSUPPORTED:
		return errors.New("The function called is not supported at this time.")
	default:
		return errors.New("Unknown error code: " + strconv.Itoa(int(result)))
	}
}

func contextDescription(context C.int) string {
	switch context {
	case C.NoError:
		return "no error context given"
	case C.LoadLibraryFailed:
		return "loading DirectSound DLL"
	case C.DirectSoundCreateMissing:
		return "DirectSoundCreate missing from DLL"
	case C.DirectSoundCreateFailed:
		return "DirectSoundCreate"
	case C.SetCooperativeLevelFailed:
		return "SetCooperativeLevel"
	case C.CreatePrimarySoundBufferFailed:
		return "creating primary sound buffer"
	case C.PrimarySetFormatFailed:
		return "setting primary format"
	case C.CreateSecondarySoundBufferFailed:
		return "creating secondary sound buffer"
	case C.PlayingSoundBufferFailed:
		return "playing sound buffer"
	case C.GlobalSoundBufferNotSet:
		return "sound buffer is nil"
	case C.GetCurrentPositionFailed:
		return "get current sound buffer position"
	case C.LockFailed:
		return "locking sound buffer"
	case C.UnlockFailed:
		return "unlocking sound buffer"
	default:
		return "unknown context"
	}
}
