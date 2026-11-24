program Liste;
type
  PKnoten = ^TKnoten;
  TKnoten = record
    Wert: Integer;
    Naechster: PKnoten
  end;
var
  Kopf: PKnoten;
  Frei: LongInt;

procedure Einfuegen(var Kopf: PKnoten; w: Integer);
var
  neu, p: PKnoten;
begin
  New(neu);
  neu^.Wert := w;
  if (Kopf = nil) or (w < Kopf^.Wert) then
  begin
    neu^.Naechster := Kopf;
    Kopf := neu
  end
  else
  begin
    p := Kopf;
    while (p^.Naechster <> nil) and (p^.Naechster^.Wert < w) do
      p := p^.Naechster;
    neu^.Naechster := p^.Naechster;
    p^.Naechster := neu
  end
end;

procedure Loesche(var Kopf: PKnoten; w: Integer);
var
  p, q: PKnoten;
begin
  p := Kopf;
  q := nil;
  while (p <> nil) and (p^.Wert <> w) do
  begin
    q := p;
    p := p^.Naechster
  end;
  if p <> nil then
  begin
    if q = nil then Kopf := p^.Naechster else q^.Naechster := p^.Naechster;
    Dispose(p)
  end
end;

procedure Zeige(p: PKnoten);
begin
  while p <> nil do
  begin
    Write(p^.Wert, ' ');
    p := p^.Naechster
  end;
  WriteLn
end;

function Laenge(p: PKnoten): Integer;
begin
  if p = nil then Laenge := 0 else Laenge := 1 + Laenge(p^.Naechster)
end;

procedure Leeren(var Kopf: PKnoten);
var
  p: PKnoten;
begin
  while Kopf <> nil do
  begin
    p := Kopf;
    Kopf := Kopf^.Naechster;
    Dispose(p)
  end
end;

begin
  Frei := MemAvail;
  Kopf := nil;
  Einfuegen(Kopf, 5);
  Einfuegen(Kopf, 2);
  Einfuegen(Kopf, 8);
  Einfuegen(Kopf, 3);
  Einfuegen(Kopf, 9);
  Zeige(Kopf);
  WriteLn('Laenge ', Laenge(Kopf));
  Loesche(Kopf, 2);
  Loesche(Kopf, 8);
  Loesche(Kopf, 7);
  Zeige(Kopf);
  WriteLn(MemAvail < Frei);
  Leeren(Kopf);
  WriteLn(Kopf = nil, ' ', MemAvail = Frei)
end.
