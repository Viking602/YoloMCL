// Pinned cubiomes e61f90580cbdd883214a8054670dacae655e59c0.
// Build with the same files/flags as legacy-reference.c. No fast-math.
#define _POSIX_C_SOURCE 200809L
#include "generator.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#ifdef _WIN32
#include <windows.h>
static double now(void) { LARGE_INTEGER t,f; QueryPerformanceCounter(&t); QueryPerformanceFrequency(&f); return (double)t.QuadPart/f.QuadPart; }
#else
#include <time.h>
static double now(void) { struct timespec t; clock_gettime(CLOCK_MONOTONIC,&t); return t.tv_sec+t.tv_nsec*1e-9; }
#endif
static const struct { const char *name; int mc,dim,flags; } cases[]={
 {"b1.7.3/Overworld",MC_B1_7,DIM_OVERWORLD,NO_BETA_OCEAN},
 {"1.6.4/Overworld",MC_1_6,DIM_OVERWORLD,0},
 {"1.7.10/Overworld",MC_1_7,DIM_OVERWORLD,0},
 {"1.12.2/Overworld",MC_1_12,DIM_OVERWORLD,0},
 {"1.16.5/Overworld",MC_1_16,DIM_OVERWORLD,0},
 {"1.17.1/Overworld",MC_1_17,DIM_OVERWORLD,0},
 {"1.20.1/Overworld",MC_1_20,DIM_OVERWORLD,0},
 {"1.20.1/Large",MC_1_20,DIM_OVERWORLD,LARGE_BIOMES},
 {"1.17.1/Nether",MC_1_17,DIM_NETHER,0},
 {"1.17.1/End",MC_1_17,DIM_END,0},
 {"1.20.1/End",MC_1_20,DIM_END,0}
};
static volatile uint64_t sink;
// Match the Go raster API's previous-leaf tie handling without disabling
// coordinate shifts (the higher-scale shortcut in genBiomes does both).
static int generate(const Generator *g,int *cache,Range r) {
 if(g->dim==DIM_OVERWORLD && g->mc>=MC_1_18) {
  uint64_t previous=0;
  for(int z=0;z<r.sz;z++)for(int x=0;x<r.sx;x++)
   cache[z*r.sx+x]=sampleBiomeNoise(&g->bn,NULL,r.x+x,r.y,r.z+z,&previous,0);
  return 0;
 }
 return genBiomes(g,cache,r);
}
static uint64_t hash(const int *ids) {
 uint64_t h=UINT64_C(14695981039346656037);
 for(int i=0;i<128*128;i++)h=(h^(uint32_t)ids[i])*UINT64_C(1099511628211);
 return h;
}
int main(int argc,char **argv) {
 int fixtures=argc>1&&!strcmp(argv[1],"--hashes");
 int init=argc>1&&!strcmp(argv[1],"--init");
 Range r={4,2048,-2048,128,128,16,1};
 if(fixtures)puts("{");else puts("case,trial,iterations,ns_per_op,hash");
 for(size_t k=0;k<sizeof(cases)/sizeof(cases[0]);k++) {
  Generator g;setupGenerator(&g,cases[k].mc,cases[k].flags);applySeed(&g,cases[k].dim,42);
  int *cache=allocCache(&g,r);if(!cache||generate(&g,cache,r))return 1;
  uint64_t h=hash(cache);
  if(fixtures) {printf("%s\"%s\":\"%016llx\"",k?",\n":"",cases[k].name,(unsigned long long)h);free(cache);continue;}
  size_t n=1;
  for(int trial=-1;trial<5;) {
   double start=now();
   for(size_t i=0;i<n;i++) {
    if(init) { setupGenerator(&g,cases[k].mc,cases[k].flags);applySeed(&g,cases[k].dim,42+(i&1023));sink^=g.seed; }
    else {if(generate(&g,cache,r))return 2;sink^=cache[0];}
   }
   double elapsed=now()-start;
   if(trial<0) {if(elapsed<0.15) {n*=2;continue;}trial=0;continue;}
   printf("%s,%d,%llu,%.3f,%016llx\n",cases[k].name,trial,(unsigned long long)n,elapsed*1e9/n,(unsigned long long)h);fflush(stdout);trial++;
  }
  free(cache);
 }
 if(fixtures)puts("\n}");
 return 0;
}
