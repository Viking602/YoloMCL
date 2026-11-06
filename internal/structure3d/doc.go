// Package structure3d loads Minecraft structure templates and jigsaw pools
// from layered jar/pack assets and renders them with a pure-Go depth buffer.
//
// The renderer produces an image.RGBA for display through mygo's Image/Bitmap
// path. Mygo's GPU backends draw a 2D scene graph only; there is no public
// mesh API, so textured 3D preview stays software-side and reusable.
package structure3d
