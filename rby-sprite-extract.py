#!/usr/bin/env python3.1
"""rby-sprite-extract.py - extract Pokemon sprites from a ROM image.

BUGS

* --color=gray doesn't work
* formats don't work very well with extract_all


"""

import struct
import io
import sys
import os, os.path
import itertools
import argparse
from os import SEEK_CUR
from struct import pack, unpack
from array import array
from subprocess import Popen, PIPE
from collections import namedtuple, OrderedDict as odict

def bitflip(x, n):
    r = 0
    while n:
        r = (r << 1) | (x & 1)
        x >>= 1
        n -= 1
    return r

class Decompressor:
    """ Pokemon sprite decompression for Gen I.

    The overall structure of this decompressor was guided by
    <http://www.magicstone.de/rhwiki/article/Grafikkomprimierung_PKMN_RGBY>,
    while the nitty-gritty was filled in by
    <http://www.upokecenter.com/projects/rbgfx.c>.

    """
    table1 = [(2 << i) - 1 for i in range(16)]

    #table2 = [
    #    [0x01, 0x32, 0x76, 0x45, 0xfe, 0xcd, 0x89, 0xba],
    #    [0xfe, 0xcd, 0x89, 0xba, 0x01, 0x32, 0x76, 0x45],
    #    [0x08, 0xc4, 0xe6, 0x2a, 0xf7, 0x3b, 0x19, 0xd5],
    #    [0xf7, 0x3b, 0x19, 0xd5, 0x08, 0xc4, 0xe6, 0x2a],
    #]
    table2 = [
        [0, 1, 3, 2, 7, 6, 4, 5, 0xf, 0xe, 0xc, 0xd, 8, 9, 0xb, 0xa],
        [0xf, 0xe, 0xc, 0xd, 8, 9, 0xb, 0xa, 0, 1, 3, 2, 7, 6, 4, 5],
    ]

    table3 = [bitflip(i, 4) for i in range(16)]

    tilesize = 8

    def __init__(self, f, mirror=False):
        self.bs = fbitstream(f)

        self.sizex = self._readint(4) * self.tilesize
        self.sizey = self._readint(4)

        self.size = self.sizex * self.sizey

        self.ramorder = next(self.bs)

        self.mirror = mirror

        self.data = None

    def decompress(self):
        rams = [[], []]

        r1 = self.ramorder
        r2 = self.ramorder ^ 1

        self._fillram(rams[r1])
        mode = self._readbit()
        if mode == 1:
            mode = 1 + self._readbit()
        self._fillram(rams[r2])

        rams[0] = bytearray(bitgroups_to_bytes(rams[0]))
        rams[1] = bytearray(bitgroups_to_bytes(rams[1]))

        if mode == 0:
            self._thing1(rams[0])
            self._thing1(rams[1])
        elif mode == 1:
            self._thing1(rams[r1])
            self._thing2(rams[r1], rams[r2])
        elif mode == 2:
            self._thing1(rams[r2], mirror=False)
            self._thing1(rams[r1])
            self._thing2(rams[r1], rams[r2])

        data = []
        for a, b in zip(bitstream(rams[0]), bitstream(rams[1])):
            data.append(a | (b << 1))

        self.data = bitgroups_to_bytes(data)

    def untile(self, mirror=None):
        if mirror is None:
            mirror = self.mirror

        ram = self.data
        out = []
        sizey = self.sizey * self.tilesize
        sizex = self.sizex // self.tilesize
        if not mirror:
            for y in range(sizey):
                for x in range(sizex):
                    k = (y + sizey * x) * 2
                    out.append(ram[k])
                    out.append(ram[k+1])
        else:
            for y in range(sizey):
                for x in reversed(range(sizex)):
                    k = (y + sizey * x) * 2
                    out.append(ram[k+1])
                    out.append(ram[k])
        return bytes(out)

    def to_image(self):
        ram = self.untile()
        bs = bitstream(ram)
        data = []
        try:
            while 1:
                data.append(readint(bs, 2))
        except StopIteration:
            pass

        img = Image((self.sizex, self.sizey * self.tilesize), data)
        return img

    def _fillram(self, ram):
        mode = ['rle', 'data'][self._readbit()]
        size = self.size * 4
        while len(ram) < size:
            if mode == 'rle':
                self._read_rle_chunk(ram)
                mode = 'data'
            elif mode == 'data':
                self._read_data_chunk(ram, size)
                mode = 'rle'
            else:
                assert False

        if len(ram) > size:
            raise ValueError(size, len(ram))
        ram[:] = self._deinterlace_bitgroups(ram)

    def _read_rle_chunk(self, ram):
        """read a run-length-encoded chunk of zeros from self.bs into `ram`"""

        # count bits until we find a 0
        i = 0
        while self._readbit():
            i += 1

        n = self.table1[i]
        a = self._readint(i+1)
        n += a

        #print(i, a, n)

        for i in range(n):
            ram.append(0)

    def _read_data_chunk(self, ram, size):
        """Read pairs of bits into `ram`"""
        while 1:
            bitgroup = self._readint(2)
            # if we encounter a pair of 0 bits, we're done
            if bitgroup == 0:
                break
            ram.append(bitgroup)

            # stop once we have enough data.
            # only matters for a handful of pokemon.
            if size <= len(ram):
                break

    def _thing1(self, ram, mirror=None):
        if mirror is None:
            mirror = self.mirror

        for x in range(self.sizex):
            bit = 0
            for y in range(self.sizey):
                i = y*self.sizex + x
                a = ram[i] >> 4 & 0xf
                b = ram[i] & 0xf

                a = self.table2[bit][a]
                bit = a & 1
                if mirror:
                    a = self.table3[a]

                b = self.table2[bit][b]
                bit = b & 1
                if mirror:
                    b = self.table3[b]

                ram[i] = (a << 4) | b

    def _thing2(self, ram1, ram2, mirror=None):
        if mirror is None:
            mirror = self.mirror

        for i in range(len(ram2)):
            if mirror:
                a = ram2[i] >> 4
                b = ram2[i] & 0xf
                a = self.table3[a]
                b = self.table3[b]
                ram2[i] = a << 4 | b

            ram2[i] ^= ram1[i]

    def _deinterlace_bitgroups(self, bits):
        l = []
        for y in range(self.sizey):
            for x in range(self.sizex):
                i = 4 * y * self.sizex + x
                for j in range(4):
                    l.append(bits[i])
                    i += self.sizex
        return l

    def _readbit(self):
        """Read a single bit."""
        return next(self.bs)

    def _readint(self, count):
        """Read an integer `count` bits in length."""
        return readint(self.bs, count)

def fbitstream(f):
    while 1:
        char = f.read(1)
        if not char:
            break
        byte = char[0]

        yield (byte >> 7) & 1
        yield (byte >> 6) & 1
        yield (byte >> 5) & 1
        yield (byte >> 4) & 1
        yield (byte >> 3) & 1
        yield (byte >> 2) & 1
        yield (byte >> 1) & 1
        yield byte & 1

def bitstream(b):
    for byte in b:
        yield (byte >> 7) & 1
        yield (byte >> 6) & 1
        yield (byte >> 5) & 1
        yield (byte >> 4) & 1
        yield (byte >> 3) & 1
        yield (byte >> 2) & 1
        yield (byte >> 1) & 1
        yield byte & 1

def readint(bs, count):
    """Read an integer `count` bits long from `bs`."""
    n = 0
    while count:
        n <<= 1
        n |= next(bs)
        count -= 1
    return n

def bitgroups_to_bytes(bits):
    l = []
    for i in range(0, len(bits)-3, 4):
        n = ((bits[i] << 6)
             | (bits[i+1] << 4)
             | (bits[i+2] << 2)
             | (bits[i+3]))
        l.append(n)
    return bytes(l)

def decompress(f, offset=None, mirror=False):
    if offset is not None:
        f.seek(offset)

    dcmp = Decompressor(f, mirror=mirror)
    dcmp.decompress()
    img = dcmp.to_image()
    return img



class Image:
    def __init__(self, size, data=None):
        self.sizex, self.sizey = size
        self.data = data
        self.palette = None

    def save_format(self, format, *args, **kw):
        formats = {
            'png': self.save_png,
            'pnm': self.save_pnm,
            'ppm': self.save_ppm,
            'pgm': self.save_pgm,
            'boxes': self.save_boxes,
            'xterm': self.save_xterm,
        }
        return formats[format](*args, **kw)

    # PIL is not yet available for python 3, so we'll write out a ppm(5) or
    # pgm(5) file and let netpbm(1) sort it out.
    def save_pam(self, out):
        def write(*args, **kw):
            print(*args, file=out, **kw)
        write("P7")
        write("HEIGHT", self.sizey)
        write("WIDTH", self.sizex)
        write("MAXVAL", 3)
        write("DEPTH", 1)
        write("TUPLTYPE", "GRAYSCALE")
        write("ENDHDR")
        i = 0
        out.flush()
        out.buffer.write(bytes(self.data))

    def save_pgm(self, out):
        if 'b' in out.mode:
            return self._save_pgm(out)
        else:
            return self._save_plain_pgm(out)

    def _save_pgm(self, out):
        out.write("P5\n{:d} {:d}\n{:d}\n"
                      .format(self.sizex, self.sizey, 3)
                      .encode())
        for byte in self.data:
            out.write(pack("<B", (3 - byte)))

    def _save_plain_pgm(self, out):
        def write(*args, **kw):
            print(*args, file=out, **kw)
        write("P2")
        write(self.sizex, self.sizey) # width, height
        write(3) # maxval
        i = 0
        width = self.sizex
        for i in range(len(self.data)):
            write(3 - self.data[i], end=" ")
            i += 1
            if i % width == 0:
                write()

    def save_ppm(self, out, *args, **kw):
        if 'b' in out.mode:
            return self._save_ppm(out, *args, **kw)
        else:
            return self._save_plain_ppm(out, *args, **kw)

    def _save_ppm(self, out, palette=None):
        if palette is None:
            palette = self.palette
        out.write("P6\n{:d} {:d}\n{:d}\n"
                      .format(self.sizex, self.sizey, 31)
                      .encode())
        for byte in self.data:
            out.write(pack("<BBB", *palette[byte]))

    def _save_plain_ppm(self, out, palette=None):
        if palette is None:
            palette = self.palette

        def write(*args, **kw):
            print(*args, file=out, **kw)
        write("P3") # magic number
        write(self.sizex, self.sizey) # width, height
        write(31) # maxval. XXX don't hardcode this

        width = self.sizex
        for i, byte in enumerate(self.data):
            write("{:2d} {:2d} {:2d}".format(*palette[byte]), end="  ")
            if (i + 1) % width == 0:
                write()

    def save_pnm(self, *args, palette=None, **kw):
        if palette is None:
            palette = self.palette

        if palette:
            return self.save_ppm(*args, palette=palette, **kw)
        else:
            return self.save_pgm(*args, **kw)

    def save_png(self, out, palette=None):
        if palette is None:
            palette = self.palette
        p = Popen("pnmtopng", stdin=PIPE, stdout=out)
        self.save_pnm(p.stdin, palette=palette)
        p.stdin.close()
        p.wait()

    def save_boxes(self, out, palette=None):
        if palette is None:
            palette = self.palette
        char_palette = "\u00a0\u2591\u2592\u2593\u2588"

        def write(*args, **kw):
            print(*args, file=out, **kw)

        width = self.sizex
        for i, byte in enumerate(self.data):
            write(char_palette[byte] * 2, end="")
            if (i + 1) % width == 0:
                write()

    def save_xterm(self, out, palette=None):
        if palette is None:
            palette = self.palette

        def write(*args, **kw):
            print(*args, file=out, **kw)

        if palette:
            colors = []
            for i, (r, g, b) in enumerate(palette):
                r = round(r / 31 * 5)
                g = round(g / 31 * 5)
                b = round(b / 31 * 5)
                color = 16 + r * 36 + g * 6 + b
                colors.append(color)

                #r = round(r / 31 * 255)
                #g = round(g / 31 * 255)
                #b = round(b / 31 * 255)
                #write("\x1b]4;%d;rgb:%2.2x/%2.2x/%2.2x\x1b\\" % (i + 16, r, g, b), end="")
            palette = colors
        else:
            palette = [231, 248, 240, 232]

        width = self.sizex
        for i, byte in enumerate(self.data):
            # xterm color escapes
            # set background color
            write("\033[48;5;{color}m".format(color=palette[byte]), end="")
            #write("\033[48;5;{color}m".format(color=byte + 16), end="")
            write("\N{NO-BREAK SPACE}" * 2, end="")
            if (i + 1) % width == 0:
                write()
        write("\033[0m")

class Palette:
    def __init__(self, colors):
        self.colors = colors

    @classmethod
    def fromfile(cls, f):
        colors = unpack("<HHHH", f.read(8))
        colors = [cls.rgb15_to_rgb(x) for x in colors]
        return cls(colors)

    @staticmethod
    def rgb15_to_rgb(v):
        r = v & 31
        g = (v >> 5) & 31
        b = (v >> 10) & 31
        return (r, g, b)

    def __getitem__(self, i):
        return self.colors[i]



Offsets = namedtuple("Offsets",
    ("base_stats base_stats_mew "
     "pokedex_order pokedex_order_length "
     "palette_map palettes "))

class Game:
    def __init__(self, rom, munge=None):
        self.rom = rom
        self._read_info()
        self._find_offsets()

        self.internal_ids = {}
        self._read_internal_ids()

        self._read_palette_map()
        if munge is not None:
            self._read_palettes(munge)
        else:
            self._read_palettes()

    def _read_info(self):
        rom = self.rom

        rom.seek(0x134)
        title = rom.read(15).rstrip(b"\x00")

        rom.seek(0x134 + 22)
        country = rom.read(1)[0]

        rom.seek(0x134 + 18)
        self.has_sgb = rom.read(1) == b"\x03"

        rom.seek(0x134 + 15)
        self.has_gbc = rom.read(1) == b"\x80"

        if title == b"POKEMON RED":
            if country == 0:
                self.version = 'red.jp'
            else:
                self.version = 'red'
        elif title == b"POKEMON GREEN":
            self.version = 'green.jp'
        elif title == b"POKEMON BLUE":
            self.version = 'blue'
        elif title == b"POKEMON YELLOW":
            self.version = 'yellow'
        else:
            raise ValueError("Unknown game", title)

        self.title = title
        self.country = country

        self.colors = ['gray']
        if self.has_sgb:
            self.colors.append('sgb')
        if self.has_gbc:
            self.colors.append('gbc')

    def _find(self, pat):
        self.rom.seek(0)
        for bank in itertools.count():
            data = self.rom.read(0x4000)
            if not data:
                break
            index = data.find(pat)
            if index != -1:
                return bank * 0x4000 + index

        raise IndexError

    def _find_offsets(self):
        # search strings
        bulbasaur_stats = pack("<BBBBBB", 1, 0x2d, 0x31, 0x31, 0x2d, 0x41)
        mew_stats = pack("<BBBBBB", 151, 100, 100, 100, 100, 100)
        palette_map = b"\x10\x16\x16\x16\x12\x12\x12\x13\x13\x13"
        pokedex_order = b"\x70\x73\x20\x23\x15\x64\x22\x50"

        offsets = {}
        offsets['base_stats'] = self._find(bulbasaur_stats)
        offsets['base_stats_mew'] = self._find(mew_stats)
        offsets['pokedex_order'] = self._find(pokedex_order)
        offsets['palette_map'] = self._find(palette_map)
        offsets['palettes'] = offsets['palette_map'] + 152

        if offsets['base_stats_mew'] > 0x8000:
            offsets['base_stats_mew'] = None

        self.offsets = Offsets(pokedex_order_length=0xbe, **offsets)

    def _read_internal_ids(self):
        self.rom.seek(self.offsets.pokedex_order)
        order = array("B", self.rom.read(self.offsets.pokedex_order_length))
        for i in range(1, 151+1):
            self.internal_ids[i] = order.index(i) + 1

    def _read_palette_map(self):
        self.rom.seek(self.offsets.palette_map)
        self.palette_map = list(self.rom.read(152))

    def _read_palettes(self, munge=True):
        self.palettes = odict()

        self.rom.seek(self.offsets.palettes)
        for color in self.colors:
            if color == 'gray':
                continue
            palettes = self.palettes[color] = []
            for i in range(40):
                palettes.append(Palette.fromfile(self.rom))
                if color == 'sgb' and munge:
                    palettes[i].colors[0] = (31, 31, 31)

    def get_palette(self, poke, color):
        return self.palettes[color][self.palette_map[poke]]

    def get_bank(self, poke):
        internal_id = self.internal_ids[poke]
        if self.offsets.base_stats_mew is not None and internal_id == 0x15:
            return 0x1
        elif internal_id == 0xb6:
            return 0xb
        elif internal_id < 0x1f:
            return 0x9
        elif internal_id < 0x4a:
            return 0xa
        elif self.version in ('red.jp', 'green.jp') and internal_id < 0x75:
            return 0xb
        elif internal_id < 0x74:
            return 0xb
        elif self.version in ('red.jp', 'green.jp') and internal_id < 0x9a:
            return 0xc
        elif internal_id < 0x99:
            return 0xc
        else:
            return 0xd

    def get_sprite_offset(self, poke, sprite='front'):
        if poke == 151 and self.offsets.base_stats_mew is not None:
            self.rom.seek(self.offsets.base_stats_mew)
        else:
            self.rom.seek(self.offsets.base_stats)
            self.rom.seek((poke - 1) * 28, SEEK_CUR)

        self.rom.seek(10, SEEK_CUR)
        size = self.rom.read(1)

        pointers = unpack("<HH", self.rom.read(4))

        if sprite == 'front':
            offset = pointers[0]
        elif sprite == 'back':
            offset = pointers[1]
        else:
            raise ValueError(sprite)

        bank = self.get_bank(poke)
        #print(poke, get_internal_id(poke), hex(bank), list(map(hex, find_bank(rom, offset, size))))
        return ((bank - 1) << 14) + offset

def find_banks(rom, pointer, size):
    banks = []
    rom.seek(pointer - 0x4000)
    for bank in range(0x3f):
        byte = rom.read(1)
        if byte == size:
            banks.append(bank)
        rom.seek(0x4000-1, SEEK_CUR)
    return banks



def extract_sprite(game, poke, color=None, sprite='front', mirror=False):
    if color is None:
        color = game.colors[-1]
    offset = game.get_sprite_offset(poke, sprite=sprite)
    #print(hex(offset), file=sys.stderr)
    img = decompress(game.rom, offset, mirror=mirror)
    img.palette = game.get_palette(poke, color)
    return img

def extract_all(game, directory, format="png"):
    basedir = directory
    ext = "." + format
    for facing in ('front', 'back'):
        for color in game.colors:
            path = construct_path(basedir, facing=facing, palette=color)
            xmakedirs(path)

        for poke in range(1, 151+1):
            offset = game.get_sprite_offset(poke, facing)

            img = decompress(game.rom, offset)
            for color in game.colors:
                path = construct_path(basedir, facing=facing, palette=color)
                path = os.path.join(path, str(poke)+ext)
                print(path)

                if color == 'gray':
                    img.palette = False
                else:
                    img.palette = game.get_palette(poke, color)

                img.save_format(format, open(path, 'wb'))

            #img.save_png(open(path, 'wb'), palette=False)

def construct_path(base, version="", facing="", palette=""):
    if facing == "front":
        facing = ""
    if palette == "sgb":
        palette = ""
    return os.path.join(base, version, facing, palette)

def xmakedirs(path):
    if not os.path.exists(path):
        os.makedirs(path)




def print_help(full=True):
    prog = os.path.basename(sys.argv[0])
    print("""\
Usage: {prog} rompath pokemon {{front|back}} [options]
       {prog} rompath -d directory [options]
       {prog} --help
""".format(prog=prog))

    if not full:
        sys.exit()

    print("""\
First form:
    Extract a single sprite to stdout

    rompath       - the path to the ROM
    pokemon       - the id of the pokemon to extract
    front | back  - frontsprite or backsprite?

Second form:
    Extract all sprites into a directory

    rompath       - the path to the ROM
    -d outdir
    --directory=  - where to put the sprites


Options:
    -c, --color=   - which palette to use (gray/sgb/gbc)
    -f, --format=  - which output format to use

    --munge
    --no-munge     - if munge is set, set the first palette entry of the
                     SGB palettes to white.  (default: munge)

Formats:
    ppm     - the ppm(1) format from netpbm(1)
    png     - the Portable Network Graphics format
    boxes   - render using unicode boxes
    xterm   - output color codes for a 256-color terminal

""".format(prog=prog))

    sys.exit()

def parse_args():
    # this is sort of a hack
    # and basically kills introspection for --help
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("-d", "--directory", action="store_true")
    parser.add_argument("-h", "--help", action="store_true")

    if not sys.argv[1:]:
        print_help(full=False)

    args, _ = parser.parse_known_args()

    if args.help:
        print_help(full=True)

    elif args.directory:
        parser_all = argparse.ArgumentParser(add_help=False)
        parser_all.add_argument("rompath")
        parser_all.add_argument("-d", "--directory")
        parser_all.add_argument("-f", "--format", default="png")
        parser_all.add_argument("--munge", action="store_true", default=True)
        parser_all.add_argument("--no-munge", dest="munge", action="store_false")
        parser_all.set_defaults(mode='all')
        return parser_all.parse_args()
    else:
        parser_single = argparse.ArgumentParser(add_help=False)
        parser_single.add_argument("rompath")
        parser_single.add_argument("pokemon", type=int)
        parser_single.add_argument("facing", nargs='?', default='front')
        parser_single.add_argument("-c", "--color")
        parser_single.add_argument("-f", "--format", default="pnm")
        parser_single.add_argument("--munge", action="store_true", default=True)
        parser_single.add_argument("--no-munge", dest="munge", action="store_false")
        parser_single.set_defaults(mode='single')
        return parser_single.parse_args()

def main():
    args = parse_args()

    f = open(args.rompath, 'rb')
    game = Game(f, args.munge)
    #print(game.offsets)

    if args.mode == 'single':
        img = extract_sprite(game, args.pokemon, sprite=args.facing, color=args.color)
        img.save_format(args.format, sys.stdout)
    elif args.mode == 'all':
        extract_all(game, args.directory, format=args.format)
    else:
        raise ValueError(args.mode)

    f.close()

main()
