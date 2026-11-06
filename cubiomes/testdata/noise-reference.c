// Compile with the pinned cubiomes checkout documented in README.md.
#include "noise.h"
#include <stdio.h>

int main(void) {
    const uint64_t seeds[] = {0,42,UINT64_MAX,UINT64_C(0x8000000000000000)};
    const double amplitudes[] = {1,0,.5,1};
    int comma=0;
    puts("[");
    for(int si=0;si<4;si++) for(int legacy=0;legacy<2;legacy++) {
        Xoroshiro xr; uint64_t lc; xSetSeed(&xr,seeds[si]); setSeed(&lc,seeds[si]);
        if(comma++) puts(",");
        printf("{\"seed\":\"%" PRId64 "\",\"legacy\":%s,\"longs\":[",(int64_t)seeds[si],legacy?"true":"false");
        for(int i=0;i<4;i++) printf("%s\"%" PRIu64 "\"",i?",":"",legacy?nextLong(&lc):xNextLong(&xr));
        xSetSeed(&xr,seeds[si]); setSeed(&lc,seeds[si]);
        PerlinNoise p; if(legacy) perlinInit(&p,&lc); else xPerlinInit(&p,&xr);
        printf("],\"improved\":%.17g",samplePerlin(&p,123.25,-.5,678.75,0,0));
        xSetSeed(&xr,seeds[si]); setSeed(&lc,seeds[si]);
        DoublePerlinNoise n; PerlinNoise octaves[8];
        if(legacy) doublePerlinInit(&n,&lc,octaves,octaves+2,-7,2);
        else xDoublePerlinInit(&n,&xr,octaves,amplitudes,-4,4,-1);
        printf(",\"normal\":%.17g}",sampleDoublePerlin(&n,123.25,-.5,678.75));
    }
    puts("\n]");
    return 0;
}
