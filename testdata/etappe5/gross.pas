program Gross;
{ Strukturen ueber 64 KB: global und auf dem Heap }
type
  TFeld = array[1..30000] of LongInt;
  PFeld = ^TFeld;
var
  Feld: TFeld;
  p: PFeld;
  Block: ^Byte;
  i: LongInt;
  Summe: LongInt;
  Vorher: LongInt;
begin
  WriteLn(SizeOf(Feld));
  for i := 1 to 30000 do Feld[i] := i;
  Summe := 0;
  for i := 1 to 30000 do Summe := Summe + Feld[i];
  WriteLn(Summe, ' ', Feld[30000]);
  Vorher := MemAvail;
  New(p);
  for i := 30000 downto 1 do p^[i] := 2 * i;
  WriteLn(p^[1], ' ', p^[30000]);
  GetMem(Block, 200000);
  FillChar(Block^, 200000, 7);
  WriteLn(Block^);
  WriteLn(Vorher - MemAvail);
  FreeMem(Block, 200000);
  Dispose(p);
  WriteLn(MemAvail = Vorher, ' ', MaxAvail > 100000000)
end.
