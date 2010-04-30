#!/usr/bin/env python3
import struct
import io
import sys
from os import SEEK_CUR
from struct import unpack
from array import array

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
                data.append(3 - readint(bs, 2))
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

    # PIL is not yet available for python 3, so we'll write out a pgm(5) file,
    # and let netpbm(1) sort it out.
    def save_pam(self, out):
        def write(*args, **kw):
            print(*args, file=out, **kw)
        write("P7")
        write("HEIGHT", sizey*4)
        write("WIDTH", sizex//2)
        write("MAXVAL", 3)
        write("DEPTH", 1)
        write("TUPLETYPE", "GRAYSCALE")
        write("ENDHDR")
        i = 0
        for _ in range(self.sizey):
            for _ in range(self.sizex):
                write(self.data[i])
                i += 1

    def save_pgm(self, out):
        def write(*args, **kw):
            print(*args, file=out, **kw)
        write("P2")
        write(self.sizex, self.sizey) # width, height
        write(3) # maxval
        i = 0
        width = self.sizex
        for i in range(len(self.data)):
            write(self.data[i], end=" ")
            """
            byte = ram[i]
            #write(byte>>6, byte>>4 & 3, byte >> 2 & 3, byte & 3, end=" ", file=out)
            write(3 - (byte>>6), 3 - (byte>>4 & 3), 3 - (byte >> 2 & 3), 3 - (byte & 3), end=" ")
            #write(3 - (byte & 3), 3 - (byte>>2 & 3), 3 - (byte >> 4 & 3), 3 - (byte >> 6), end=" ", file=out)
            """
            i += 1
            if i % width == 0:
                write()



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
ORDER_LENGTH = 0xbe

internal_ids = {}
def read_pokedex_order(rom):
    rom.seek(ORDER_OFFSET)
    order = array("B", rom.read(ORDER_LENGTH))
    for i in range(1, 151+1):
        internal_ids[i] = order.index(i) + 1

def get_internal_id(poke):
    return internal_ids[poke]

def get_bank(poke):
    internal_id = get_internal_id(poke)
    if internal_id == 0x15:
        return 1
    elif internal_id == 0x15:
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
    if poke == 151:
        rom.seek(MEW_OFFSET)
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


def extract_sprite(rom, poke, sprite='front'):
    offset = get_offset(rom, poke, sprite=sprite)
    print(hex(offset), file=sys.stderr)
    img = decompress(rom, offset)
    return img

f = open("../../red.gb", 'rb')
read_pokedex_order(f)
img = extract_sprite(f, 151)
img.save_pgm(sys.stdout)
