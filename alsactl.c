#include <stdlib.h>
#include <alsa/asoundlib.h>
#include <alsa/mixer.h>

#include "alsactl.h"

static const char* card = "default";

void* alsactl_open() {
    snd_mixer_t* handle;
    snd_mixer_open(&handle, 0);
    snd_mixer_attach(handle, card);
    snd_mixer_selem_register(handle, NULL, NULL);
    snd_mixer_load(handle);

    return handle;
}

void alsactl_close(void* handle) {
    snd_mixer_close((snd_mixer_t*)handle);
}

static snd_mixer_elem_t* alsactl_find_selem(void* handle, char* name)
{
    snd_mixer_selem_id_t* sid;
    snd_mixer_selem_id_alloca(&sid);
    snd_mixer_selem_id_set_index(sid, 0);
    snd_mixer_selem_id_set_name(sid, name);

    snd_mixer_elem_t* elem = snd_mixer_find_selem((snd_mixer_t*)handle, sid);
    free(name); // was malloced from go src by C.CString()
    return elem;
}

int alsactl_get_volume(void* handle, char* name, long* min, long* max, long* val)
{
    snd_mixer_elem_t* elem = alsactl_find_selem(handle, name);
    if (! elem)
      return 1;
    
    snd_mixer_selem_get_playback_volume_range(elem, min, max);
    snd_mixer_selem_get_playback_volume(elem, SND_MIXER_SCHN_FRONT_LEFT, val);
    
    //snd_mixer_selem_set_playback_volume_all(elem, volume * max / 100);

    return 0;
}

int alsactl_get_switch(void* handle, char* name, int* val)
{
    snd_mixer_elem_t* elem = alsactl_find_selem(handle, name);
    if (! elem)
      return 1;
    
    snd_mixer_selem_get_playback_switch(elem, SND_MIXER_SCHN_FRONT_LEFT, val);

    return 0;
}

int alsactl_get_enum(void* handle, char* name, char** result)
{
    snd_mixer_elem_t* elem = alsactl_find_selem(handle, name);
    if (! elem)
      return 1;

    unsigned int val;
    *result = (char*) malloc(32);
    snd_mixer_selem_get_enum_item(elem, SND_MIXER_SCHN_FRONT_LEFT, &val);
    snd_mixer_selem_get_enum_item_name(elem, val, 32, *result);

    return 0;
}
