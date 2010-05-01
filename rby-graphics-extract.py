#!/usr/bin/env python3
import struct
import io
import sys
from os import SEEK_CUR
from struct import pack, unpack
from array import array
from subprocess import Popen, PIPE

def bitflip(x, n):
    r = 0
    while n:
        r = (r << 1) | (x & 1)
        x >>= 1
        n -= 1
    return r

class Decompressor:
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

class Image:
    def __init__(self, size, data=None):
        self.sizex, self.sizey = size
        self.data = data
        self.palette = None

    # PIL is not yet available for python 3, so we'll write out a pgm(5) file,
    # and let netpbm(1) sort it out.
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

BASE_STATS_OFFSET = 0x383de
MEW_OFFSET = 0x425b
ORDER_OFFSET = 0x41024
ORDER_OFFSET_Y = 0x410b1
ORDER_LENGTH = 0xbe
PALETTE_OFFSET = 0x725c8
PALETTE_OFFSET_Y = 0x72921

internal_ids = {}
def read_pokedex_order(rom):
    offset = ORDER_OFFSET_Y if version == 'yellow' else ORDER_OFFSET
    rom.seek(offset)
    order = array("B", rom.read(ORDER_LENGTH))
    for i in range(1, 151+1):
        internal_ids[i] = order.index(i) + 1

def get_internal_id(poke):
    return internal_ids[poke]

porder = palettes = None
def read_palettes(rom):
    global porder, palettes
    offset = PALETTE_OFFSET_Y if version == 'yellow' else PALETTE_OFFSET
    rom.seek(offset)
    porder = rom.read(152)
    palettes = []
    for i in range(0x20):
        palettes.append(Palette.fromfile(rom))

def get_palette(poke):
    return palettes[porder[poke]]


def get_bank(poke):
    internal_id = get_internal_id(poke)
    if version in ('red', 'blue') and internal_id == 0x15:
        return 0x1
    elif internal_id == 0xb6:
        return 0xb
    elif internal_id < 0x1f:
        return 0x9
    elif internal_id < 0x4a:
        return 0xa
    elif internal_id < 0x74:
        return 0xb
    elif internal_id < 0x99:
        return 0xc
    else:
        return 0xd

def get_offset(rom, poke, sprite='front'):
    bank = get_bank(poke)
    if version in ('red', 'blue') and poke == 151:
        rom.seek(MEW_OFFSET)
        #rom.seek(BASE_STATS_OFFSET)
        #rom.seek((poke - 1) * 28, SEEK_CUR)
    else:
        rom.seek(BASE_STATS_OFFSET)
        rom.seek((poke - 1) * 28, SEEK_CUR)
    rom.seek(11, SEEK_CUR)

    offsets = unpack("<HH", rom.read(4))

    if sprite == 'front':
        offset = offsets[0]
    elif sprite == 'back':
        offset = offsets[1]
    else:
        raise ValueError(sprite)

    return ((bank - 1) << 14) + offset

def get_version(rom):
    rom.seek(0x134)
    title = rom.read(16).rstrip(b"\x00\x80")
    if title == b"POKEMON RED":
        return 'red'
    elif title == b"POKEMON BLUE":
        return 'blue'
    elif title == b"POKEMON YELLOW":
        return 'yellow'

    raise ValueError("Unknown game", title)


def extract_sprite(rom, poke, sprite='front', mirror=False):
    offset = get_offset(rom, poke, sprite=sprite)
    #print(hex(offset), file=sys.stderr)
    img = decompress(rom, offset, mirror=mirror)
    img.palette = get_palette(poke)
    return img



rompath = sys.argv[1]
pokemon = int(sys.argv[2])
try:
    sprite = sys.argv[3]
except LookupError:
    sprite = 'front'

f = open(rompath, 'rb')
version = get_version(f)
read_pokedex_order(f)
read_palettes(f)

img = extract_sprite(f, pokemon, sprite=sprite)
img.save_png(sys.stdout)
#img.save_ppm(sys.stdout)
f.close()
