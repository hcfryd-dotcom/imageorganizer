// Package analysis versions each kind of per-image analysis. Every image
// records which version of each component produced its data; bumping a
// version here makes the matching stage redo exactly the images that are
// behind, and nothing else.
//
// Components that need the original photo (Media, Faces) are redone by the
// import stage, which reads it once more. The others work from local derived
// data (thumbnails, face crops) and never touch the source again.
package analysis

const (
	// Media: EXIF metadata, thumbnail, perceptual hash. Needs the original.
	Media = 1
	// Faces: detection, landmarks, identity embeddings, face crops. Needs
	// the original.
	Faces = 1
	// Embed: the image's search embedding, computed from the thumbnail.
	// (Changing the embedding model also re-indexes, independently.)
	Embed = 1
	// Age: apparent age per face. AgeFromCrop is the fallback estimate from
	// the stored face crop (face only); AgeFull is estimated during import
	// from the full photo, with the body and a mirrored pass, which is more
	// precise. A face's age version records which one produced it.
	AgeFromCrop = 1
	AgeFull     = 2
	// Nsfw: how likely the photo is not safe for work (nudity), from the
	// thumbnail. 2: the thumbnail is smoothed when shrunk, as the model expects.
	Nsfw = 2
	// Blur: how sharp the photo is, from the thumbnail.
	Blur = 1
)

// Component names as stored on images.
const (
	CMedia = "media"
	CFaces = "faces"
	CEmbed = "embed"
	CNsfw  = "nsfw"
	CBlur  = "blur"
)

// FromOriginal lists components that require reading the original file.
var FromOriginal = map[string]int{CMedia: Media, CFaces: Faces}

// Versions records the analysis versions applied to an image.
type Versions map[string]int

// Behind reports whether component c is older than want.
func (v Versions) Behind(c string, want int) bool { return v[c] < want }

// NeedsOriginal reports whether any component that needs the original file
// is out of date. faces=false skips the face component (face analysis off).
func (v Versions) NeedsOriginal(faces bool) bool {
	if v.Behind(CMedia, Media) {
		return true
	}
	return faces && v.Behind(CFaces, Faces)
}
