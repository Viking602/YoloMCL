/* Reference: Cubitect/cubiomes e61f90580cbdd883214a8054670dacae655e59c0 (MIT).
 * Link generator.c biomenoise.c noise.c layers.c biomes.c with -O3 -lm. */
#include "generator.h"
#include <stdio.h>
#include <stdlib.h>
#include <inttypes.h>
static uint64_t state=0x123456789abcdef0ULL;
static uint32_t draw(void) {state=state*6364136223846793005ULL+1442695040888963407ULL;return state>>32;}
static uint64_t digest(int *p,int n) {
    uint64_t h=14695981039346656037ULL;
    for(int i=0;i<n;i++) h=(h^(uint32_t)p[i])*1099511628211ULL;
    return h;
}
/* The pinned mapVoronoiPlane helper does not agree with voronoiAccess3D:
 * its Y seed coordinates and fixed half-quart distance differ, and mapVoronoi
 * subtracts the horizontal offset twice. Compose the documented 3D access
 * primitive with the quart generator for 1.15+ block queries instead. */
static int blockBiome(const Generator *g,int x,int y,int z) {
    if(g->mc>=MC_1_15) {
        int a,b,c;voronoiAccess3D(g->sha,x,y,z,&a,&b,&c);
        return getBiomeAt(g,4,a,b,c);
    }
    return getBiomeAt(g,1,x,y,z);
}
static void blockRange(const Generator *g,int *out,Range r) {
    Range s=getVoronoiSrcRange(r);
    int *src=allocCache(g,s);if(!src||genBiomes(g,src,s))exit(4);
    for(int j=0;j<r.sz;j++)for(int i=0;i<r.sx;i++) {
        int x,y,z;voronoiAccess3D(g->sha,r.x+i,r.y,r.z+j,&x,&y,&z);
        out[j*r.sx+i]=src[((y-s.y)*s.sz+z-s.z)*s.sx+x-s.x];
    }
    free(src);
}
int main(void) {
    const struct {const char *name;int mc;} versions[]={
		{"b1.7.3",MC_B1_7},
        {"b1.8.1",MC_B1_8},{"1.0",MC_1_0},{"1.1",MC_1_1},{"1.2.5",MC_1_2},
        {"1.3.2",MC_1_3},{"1.4.7",MC_1_4},{"1.5.2",MC_1_5},{"1.6.4",MC_1_6},
        {"1.7.10",MC_1_7},{"1.8.9",MC_1_8},{"1.9.4",MC_1_9},{"1.10.2",MC_1_10},
        {"1.11.2",MC_1_11},{"1.12.2",MC_1_12},{"1.13.2",MC_1_13},{"1.14.4",MC_1_14},
        {"1.15.2",MC_1_15},{"1.16.1",MC_1_16_1},{"1.16.5",MC_1_16},{"1.17.1",MC_1_17}
    };
    const int64_t seeds[]={0,1,42,123456789,-1,INT64_MIN,INT64_MAX};
    int comma=0;puts("[");
    for(unsigned v=0;v<sizeof(versions)/sizeof(*versions);v++)
    for(unsigned si=0;si<sizeof(seeds)/sizeof(*seeds);si++)
    for(int dim=-1;dim<=1;dim++) {
        if(dim==1&&versions[v].mc<MC_1_0)continue;
        for(int large=0;large<=1;large++) {
            if(large&&(dim!=0||versions[v].mc<MC_1_3))continue;
            Generator g;setupGenerator(&g,versions[v].mc,(large?LARGE_BIOMES:0)|NO_BETA_OCEAN);applySeed(&g,dim,seeds[si]);
            printf("%s{\"version\":\"%s\",\"seed\":%"PRId64",\"dimension\":%d,\"large\":%s,\"points\":[",comma?",\n":"",versions[v].name,seeds[si],dim,large?"true":"false");comma=1;
            for(int i=0;i<64;i++) {
                int x,z,y=(int)(draw()%768)-128;
                if(i<16){x=i-9;z=5-i;}else if(i<48){x=(int)(draw()%2000001)-1000000;z=(int)(draw()%2000001)-1000000;}else{x=(int)(draw()%60000001)-30000000;z=(int)(draw()%60000001)-30000000;}
                int a=getBiomeAt(&g,4,x>>2,y>>2,z>>2),b=blockBiome(&g,x,y,z);
                if(a<0||b<0)return 2;
                printf("%s[%d,%d,%d,%d,%d]",i?",":"",x,y,z,a,b);
            }
            printf("],\"ranges\":[");
            for(int ri=0;ri<4;ri++) {
                int scale=ri&1?1:4;
                Range r={scale,ri<2?-17:2049,ri<2?-23:-1023,ri<2?47:128,ri<2?41:128,16,1};
                int *p=allocCache(&g,r);if(!p||genBiomes(&g,p,r))return 3;
                if(scale==1&&g.mc>=MC_1_15)blockRange(&g,p,r);
                printf("%s{\"scale\":%d,\"x\":%d,\"y\":%d,\"z\":%d,\"w\":%d,\"h\":%d,\"hash\":\"%016"PRIx64"\"}",ri?",":"",scale,r.x,r.y,r.z,r.sx,r.sz,digest(p,r.sx*r.sz));free(p);
            }
            printf("]}");
        }
    }
    puts("\n]");return 0;
}
