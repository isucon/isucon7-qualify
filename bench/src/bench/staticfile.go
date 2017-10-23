package bench

type StaticFile struct {
	Path string
	Size int64
	Hash string
}

var (
	StaticFiles = []*StaticFile{
		&StaticFile{"/css/bootstrap.min.css", 150996, "7e923ad223e9f33e54d22e50cf2bcce5"},
		&StaticFile{"/css/main.css", 1741, "d8c2a974e5816bd9f839f84a77348970"},
		&StaticFile{"/favicon.ico", 318, "7157dc4688c274fe0bc2e3122cac19c9"},
		&StaticFile{"/fonts/glyphicons-halflings-regular.eot", 20127, "f4769f9bdb7466be65088239c12046d1"},
		&StaticFile{"/fonts/glyphicons-halflings-regular.svg", 108738, "89889688147bd7575d6327160d64e760"},
		&StaticFile{"/fonts/glyphicons-halflings-regular.ttf", 45404, "e18bbf611f2a2e43afc071aa2f4e1512"},
		&StaticFile{"/fonts/glyphicons-halflings-regular.woff", 23424, "fa2772327f55d8198301fdb8bcfc8158"},
		&StaticFile{"/fonts/glyphicons-halflings-regular.woff2", 18028, "448c34a56d699c29117adc64c43affeb"},
		&StaticFile{"/js/bootstrap.min.js", 46653, "0827a0bdcd9a917990eee461a77dd33e"},
		&StaticFile{"/js/chat.js", 4162, "c557e68d34fdfb347fa4cf00e1eba7bd"},
		&StaticFile{"/js/jquery.min.js", 86659, "c9f5aeeca3ad37bf2aa006139b935f0a"},
		&StaticFile{"/js/tether.min.js", 24632, "1c4a5999a2b43cdd3aaa88a04f24c961"},
	}
)
