program TypisierteDateien;
type
  TEintrag = record
    Nr: Integer;
    Name: string[30];
    Preis: Real
  end;
var
  f: file of TEintrag;
  g: file;
  e: TEintrag;
  Puffer: array[1..512] of Byte;
  Gelesen: Word;
  i: Integer;
begin
  Assign(f, 'artikel.dat');
  Rewrite(f);
  for i := 1 to 3 do
  begin
    e.Nr := i;
    e.Name := 'Artikel';
    e.Preis := i * 9.99;
    Write(f, e)
  end;
  Seek(f, 1);
  Read(f, e);
  WriteLn(e.Nr, ' ', e.Name, ' ', e.Preis:0:2, ' von ', FileSize(f), ' an ', FilePos(f));
  Close(f);

  Assign(g, 'artikel.dat');
  Reset(g, 1);
  BlockRead(g, Puffer, SizeOf(Puffer), Gelesen);
  WriteLn(Gelesen, ' Bytes');
  Close(g);
  Rename(g, 'alt.dat')
end.
