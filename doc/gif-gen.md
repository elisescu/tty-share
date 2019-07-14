# Generate the gif from video

``` bash
$ ffmpeg -v
ffmpeg version 3.3 Copyright (c) 2000-2017 the FFmpeg developers
...

$ ffmpeg -i input.mov -vf  fps=15,scale=1920:-1:flags=lanczos output.gif
```
