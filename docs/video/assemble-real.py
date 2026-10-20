#!/usr/bin/env python3
"""Compose synchronized physical-device captures; never generates demonstration UI.

python docs/video/assemble-real.py capture.json [--dry-run]
Input/output paths are relative to capture.json. Existing outputs are never replaced.
"""
import argparse
import json
import math
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


FLOWS = {
    "windows-text", "windows-image", "android-text-pwa",
    "android-text-tile", "android-image",
}


def number(value, name, minimum=0):
    if isinstance(value, bool) or not isinstance(value, (float, int)):
        raise ValueError(f"{name} must be a number")
    if not math.isfinite(value) or value < minimum:
        raise ValueError(f"Invalid {name}")
    return value


def validate(data):
    if data.get("flow") not in FLOWS:
        raise ValueError("Unknown flow")
    for field in ("release", "windows", "android"):
        value = data.get(field)
        if not isinstance(value, str) or not value.strip() or "REPLACE_" in value:
            raise ValueError(f"Record actual {field} metadata first")
    duration = number(data.get("duration"), "duration", 15)
    if duration > 40:
        raise ValueError("Everyday clips must be 15–40 seconds")
    for source in ("desktop", "phone"):
        config = data.get(source, {})
        if not isinstance(config.get("file"), str) or not config["file"]:
            raise ValueError(f"Missing {source} input file")
        number(config.get("start"), f"{source}.start")
        crop = config.get("crop")
        if crop is not None and (
            not isinstance(crop, list) or len(crop) != 4
            or any(isinstance(x, bool) or not isinstance(x, int) for x in crop)
            or min(crop[:2]) < 2 or min(crop[2:]) < 0
        ):
            raise ValueError(f"{source}.crop must be [width, height, x, y]")
    previous = 0
    captions = data.get("captions")
    if not isinstance(captions, list) or not captions:
        raise ValueError("Captions are required")
    for caption in captions:
        start = number(caption.get("start"), "caption.start")
        end = number(caption.get("end"), "caption.end")
        if start < previous or end <= start or end > duration:
            raise ValueError("Caption times must be ordered, non-overlapping and within clip")
        text = caption.get("text")
        if not isinstance(text, str) or not text.strip() or any(c in text for c in "{}\\\r"):
            raise ValueError("Caption text must be plain text (newlines allowed, no ASS markup)")
        previous = end
    font = data.get("font", "Arial")
    if not isinstance(font, str) or not font or any(c in font for c in ",\r\n"):
        raise ValueError("Invalid font name")
    if not isinstance(data.get("output"), str) or Path(data["output"]).suffix.lower() != ".mp4":
        raise ValueError("Output must be an MP4 file")


def ass_time(seconds):
    ticks = round(seconds * 100)
    return f"{ticks // 360000}:{ticks // 6000 % 60:02}:{ticks // 100 % 60:02}.{ticks % 100:02}"


def subtitles(data):
    # Encoding=177 requests Hebrew; libass handles bidi and shaping. Keep captions
    # plain text; the renderer controls every ASS override.
    header = f"""[Script Info]
ScriptType: v4.00+
PlayResX: 1080
PlayResY: 1920
WrapStyle: 0

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Caption,{data.get('font', 'Arial')},54,&H00FFFFFF,&H00FFFFFF,&H00202020,&H00202020,0,0,0,0,100,100,0,0,1,2,0,2,45,45,85,177
Style: Label,{data.get('font', 'Arial')},36,&H00FFFFFF,&H00FFFFFF,&H00202020,&H00202020,0,0,0,0,100,100,0,0,1,2,0,8,30,30,12,177

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
"""
    end = ass_time(data["duration"])
    events = [f"Dialogue: 0,0:00:00.00,{end},Label,,0,0,0,,מחשב",
              f"Dialogue: 0,0:00:00.00,{end},Label,,0,0,0,,{{\\pos(540,700)}}טלפון"]
    for caption in data["captions"]:
        text = caption["text"].replace("\n", r"\N")
        events.append(f"Dialogue: 1,{ass_time(caption['start'])},{ass_time(caption['end'])},Caption,,0,0,0,,{text}")
    return header + "\n".join(events) + "\n"


def filters(data):
    parts = []
    for index, (name, width, height) in enumerate((("desktop", 1080, 608), ("phone", 506, 900))):
        crop = data[name].get("crop")
        prefix = f"crop={':'.join(map(str, crop))}," if crop else ""
        parts.append(f"[{index}:v]setpts=PTS-STARTPTS,{prefix}"
                     f"scale={width}:{height}:force_original_aspect_ratio=decrease,"
                     f"pad={width}:{height}:(ow-iw)/2:(oh-ih)/2:color=0x161c29,"
                     f"setsar=1,fps=30[{name}]")
    parts += [f"color=c=0x161c29:s=1080x1920:r=30:d={data['duration']}[background]",
              "[background][desktop]overlay=0:60:shortest=1[upper]",
              "[upper][phone]overlay=287:750:shortest=1,ass=captions.ass[v]"]
    return ";".join(parts)


def probe(path):
    result = subprocess.run(["ffprobe", "-v", "error", "-show_streams", "-show_format",
                             "-of", "json", str(path)], check=True, capture_output=True, text=True)
    return json.loads(result.stdout)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--dry-run", action="store_true", help="Check metadata and print filters; do not read media")
    args = parser.parse_args()
    manifest = args.manifest.resolve()
    data = json.loads(manifest.read_text(encoding="utf-8"))
    validate(data)
    if args.dry_run:
        print("Metadata valid. Dry run does NOT verify physical captures or media.")
        print(filters(data))
        return
    if data.get("confirmed_real_capture") is not True:
        raise ValueError("Review both physical-device recordings and set confirmed_real_capture=true first")
    if not all(shutil.which(tool) for tool in ("ffmpeg", "ffprobe")):
        raise ValueError("ffmpeg and ffprobe must be on PATH")
    inputs = [(manifest.parent / data[name]["file"]).resolve() for name in ("desktop", "phone")]
    output = (manifest.parent / data["output"]).resolve()
    if output.exists():
        raise ValueError("Output already exists; choose another output filename")
    for name, path in zip(("desktop", "phone"), inputs):
        media = probe(path)
        videos = [s for s in media["streams"] if s["codec_type"] == "video"]
        if not videos:
            raise ValueError(f"No video stream in {name}")
        # Prefer video duration, so a longer audio track cannot hide a short capture.
        duration = videos[0].get("duration", media.get("format", {}).get("duration"))
        if duration is None or float(duration) + .05 < data[name]["start"] + data["duration"]:
            raise ValueError(f"{name} recording is shorter than the requested timeline")
        crop = data[name].get("crop")
        if crop and (crop[0] + crop[2] > videos[0]["width"] or crop[1] + crop[3] > videos[0]["height"]):
            raise ValueError(f"{name} crop exceeds input dimensions")
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="copyapp-real-") as work:
        Path(work, "captions.ass").write_text(subtitles(data), encoding="utf-8")
        Path(work, "filter.txt").write_text(filters(data), encoding="utf-8")
        command = ["ffmpeg", "-n", "-hide_banner", "-loglevel", "warning"]
        for name, path in zip(("desktop", "phone"), inputs):
            command += ["-ss", str(data[name]["start"]), "-t", str(data["duration"]), "-i", str(path)]
        command += ["-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-filter_complex_script", "filter.txt",
                    "-map", "[v]", "-map", "2:a", "-t", str(data["duration"]), "-shortest",
                    "-c:v", "libx264", "-crf", "23", "-preset", "medium", "-pix_fmt", "yuv420p",
                    "-r", "30", "-c:a", "aac", "-b:a", "64k", "-movflags", "+faststart", str(output)]
        subprocess.run(command, cwd=work, check=True)
    media = probe(output)
    video = next(s for s in media["streams"] if s["codec_type"] == "video")
    audio = next(s for s in media["streams"] if s["codec_type"] == "audio")
    if (video["codec_name"], video["width"], video["height"], video["pix_fmt"], audio["codec_name"]) != (
            "h264", 1080, 1920, "yuv420p", "aac"):
        raise ValueError("Output failed codec/dimensions check; do not publish")
    if abs(float(video.get("duration", media["format"]["duration"])) - data["duration"]) > .15:
        raise ValueError("Output duration is incomplete; do not publish")
    print(f"Created {output}. Review Hebrew RTL, transfer proof and WhatsApp playback before publishing.")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print(f"Error: {error}", file=sys.stderr)
        sys.exit(1)
