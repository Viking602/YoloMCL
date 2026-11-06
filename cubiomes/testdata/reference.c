#include "biomenoise.h"
#include <stdio.h>

int main(void) {
    const uint64_t seeds[] = {0,1,42,123456789,UINT64_MAX,UINT64_C(0x8000000000000000),UINT64_C(0x7fffffffffffffff)};
    const int coords[][3] = {{0,64,0},{100,64,100},{-100,64,-100},{1000,64,-1000},{-12345,64,6789},{256,-32,256},{-4096,160,8192},{29900000,64,-29900000},{-1,0,-1}};
    printf("[\n");
    int comma=0;
    for (int version=0;version<6;version++) for (int si=0;si<7;si++) {
        const int versions[] = {MC_1_18,MC_1_19,MC_1_20,MC_1_21,MC_1_19_2,MC_1_20};
        const char *names[] = {"1.18.2","1.19.4","1.20.1","1.21.4","1.19.2","1.20.1"};
        BiomeNoise bn; initBiomeNoise(&bn,versions[version]);
        setBiomeSeed(&bn,seeds[si],version==5);
        NetherNoise nn; setNetherSeed(&nn,seeds[si]);
        for (int ci=0;ci<9;ci++) {
            int x=coords[ci][0],y=coords[ci][1],z=coords[ci][2]; int64_t np[6];
            int id=sampleBiomeNoise(&bn,np,x>>2,y>>2,z>>2,NULL,0);
            if(comma++) printf(",\n");
            printf("{\"version\":\"%s\",\"large\":%s,\"seed\":\"%" PRId64 "\",\"x\":%d,\"y\":%d,\"z\":%d,\"biome\":%d,\"climate\":[",names[version],version==5?"true":"false",(int64_t)seeds[si],x,y,z,id);
            for(int k=0;k<6;k++) printf("%s%" PRId64,k?",":"",np[k]);
            printf("],\"nether\":%d}",getNetherBiome(&nn,x>>2,y>>2,z>>2,NULL));
        }
    }
    printf("\n]\n");
    return 0;
}
