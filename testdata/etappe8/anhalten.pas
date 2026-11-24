program Anhalten;
{ Anhalten tief in einer Rekursion: veraenderte Wertparameter, lokale
  Arrays und Records, Strings, with, Funktionsergebnisse und eine innere
  Prozedur (Static Link) muessen das Abwickeln und Fortsetzen ueberstehen. }
uses Crt;
type
  TPunkt = record
    x, y: Integer
  end;
var
  Global: Integer;

function Tief(n: Integer; s: string): Integer;
var
  Feld: array[1..3] of Integer;
  i: Integer;
  p: TPunkt;

  procedure Innen;
  begin
    Delay(1);
    Inc(Global, n)
  end;

begin
  n := n * 2;
  for i := 1 to 3 do
    Feld[i] := n + i;
  p.x := n;
  p.y := -n;
  with p do
  begin
    Delay(1);
    x := x + 1
  end;
  if n < 16 then
    Tief := Tief(n div 2 + 1, s + '.') + Feld[3] + p.x
  else
  begin
    Innen;
    Tief := Feld[1] + Length(s)
  end;
  for i := 1 to 2 do
  begin
    Delay(1);
    Write(n, ':', i, ' ')
  end
end;

begin
  Global := 0;
  WriteLn(Tief(1, 'x'));
  WriteLn(Global)
end.
