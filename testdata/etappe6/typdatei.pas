program TypDatei;
type
  TArtikel = record
    Nr: Integer;
    Name: string[15];
    Preis: Real
  end;
var
  f: file of TArtikel;
  a: TArtikel;
  i: Integer;
  g: file;
  Puffer: array[1..10] of Byte;
  Zahlen: file of Integer;
  z, Gelesen: Integer;
begin
  Assign(f, 'artikel.dat');
  Rewrite(f);
  for i := 1 to 4 do
  begin
    a.Nr := i;
    a.Name := 'Artikel ' + Chr(Ord('A') + i - 1);
    a.Preis := i * 1.25;
    Write(f, a)
  end;
  WriteLn('Groesse: ', FileSize(f), ' Position: ', FilePos(f));
  Seek(f, 2);
  Read(f, a);
  WriteLn(a.Nr, ' ', a.Name, ' ', a.Preis:0:2, ' Position: ', FilePos(f));
  Seek(f, 1);
  Truncate(f);
  WriteLn('nach Truncate: ', FileSize(f), ' ', Eof(f));
  Close(f);
  Reset(f);
  while not Eof(f) do
  begin
    Read(f, a);
    Write(a.Name, ';')
  end;
  WriteLn;
  Close(f);

  Assign(Zahlen, 'zahlen.dat');
  Rewrite(Zahlen);
  for i := 1 to 5 do
    Write(Zahlen, i * i);
  Close(Zahlen);
  Assign(g, 'zahlen.dat');
  Reset(g, 1);
  BlockRead(g, Puffer, 10, Gelesen);
  WriteLn(Gelesen, ' ', Puffer[1], ' ', Puffer[5], ' ', Puffer[9]);
  BlockRead(g, Puffer, 10, Gelesen);
  WriteLn(Gelesen, ' ', Puffer[3], ' ', Puffer[7]);
  BlockRead(g, Puffer, 10, Gelesen);
  WriteLn(Gelesen);
  Close(g);
  Rename(g, 'quadrate.dat');
  Assign(Zahlen, 'quadrate.dat');
  Reset(Zahlen);
  Seek(Zahlen, 4);
  Read(Zahlen, z);
  WriteLn(z, ' ', FileSize(Zahlen));
  Close(Zahlen);
  Erase(Zahlen)
end.
