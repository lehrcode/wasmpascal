program Typen;
type
  TFarbe = (Rot, Gruen, Blau);
  TKlein = 0..9;
var
  b: Byte;
  k: ShortInt;
  w: Word;
  i: Integer;
  l: LongInt;
  c: Char;
  f: TFarbe;
  ok: Boolean;
  t: TKlein;
  z: Integer;
begin
  b := 255;
  b := b + 1;
  WriteLn('Byte 255+1 = ', b);
  b := 200;
  i := b + b;
  WriteLn('200+200 als Integer = ', i);
  k := 127;
  k := k + 1;
  WriteLn('ShortInt 127+1 = ', k);
  w := 65535;
  w := w + 1;
  WriteLn('Word 65535+1 = ', w);
  i := MaxInt;
  i := i + 1;
  WriteLn('MaxInt+1 = ', i);
  l := -7;
  WriteLn(l div 2, ' ', l mod 2, ' ', 7 div -2, ' ', 7 mod -2);
  WriteLn(1 shl 10, ' ', -1 shr 28, ' ', $F0 and $3C, ' ', $F0 or $0F, ' ', $FF xor $0F, ' ', not 0);
  WriteLn(Hi($1234), ' ', Lo($1234), ' ', Swap($1234));
  c := 'A';
  c := Succ(c);
  WriteLn(c, Ord(c), Chr(Ord(c) + 1), Pred('z'));
  i := 300;
  c := Chr(i);
  WriteLn(Ord(c));
  f := Gruen;
  WriteLn(Ord(f), ' ', Ord(Succ(f)), ' ', Ord(High(TFarbe)));
  ok := f = Gruen;
  WriteLn(ok, ' ', not ok, ' ', ok and (f <> Rot), ' ', ok xor True);
  WriteLn(Byte(300), ' ', ShortInt(200), ' ', Word(-1), ' ', Ord(TFarbe(2)));
  t := 9;
  WriteLn(t, ' ', SizeOf(t), ' ', SizeOf(w), ' ', SizeOf(Real));
  z := 0;
  if (z <> 0) and (10 div z > 1) then WriteLn('nie') else WriteLn('Kurzschluss ok');
  if (z = 0) or (10 div z > 1) then WriteLn('Kurzschluss ok');
  ok := (z = 0) or (10 div z = 1);
  WriteLn(ok);
  WriteLn(5 in [1..3, 5], ' ', 'x' in ['a'..'f'], ' ', Abs(-17), ' ', Sqr(-4))
end.
