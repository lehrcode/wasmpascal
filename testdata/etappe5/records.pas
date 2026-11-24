program Records;
type
  TPunkt = record
    x, y: Integer
  end;
  TRechteck = record
    LinksOben, RechtsUnten: TPunkt;
    Name: string[10]
  end;
  TForm = (Kreis, Quadrat);
  TFigur = record
    Mitte: TPunkt;
    case Art: TForm of
      Kreis: (Radius: Integer);
      Quadrat: (Seite: Integer; Gedreht: Boolean)
  end;
  TPolygon = record
    Anzahl: Integer;
    Ecken: array[1..4] of TPunkt
  end;
var
  p, q: TPunkt;
  r: TRechteck;
  f: TFigur;
  poly: TPolygon;
  i: Integer;

function Punkt(x, y: Integer): TPunkt;
begin
  Result.x := x;
  Result.y := y
end;

function Flaeche(const r: TRechteck): Integer;
begin
  with r do
    Flaeche := (RechtsUnten.x - LinksOben.x) * (RechtsUnten.y - LinksOben.y)
end;

procedure Verschiebe(var p: TPunkt; dx, dy: Integer);
begin
  p.x := p.x + dx;
  p.y := p.y + dy
end;

procedure Kopie(p: TPunkt);
begin
  p.x := 999;
  Write(p.x, ' ')
end;

begin
  p := Punkt(1, 2);
  q := p;
  q.x := 10;
  WriteLn(p.x, ' ', p.y, ' ', q.x, ' ', q.y);
  Kopie(p);
  WriteLn(p.x);
  Verschiebe(p, 5, 5);
  WriteLn(p.x, ',', p.y);
  r.LinksOben := Punkt(0, 0);
  r.RechtsUnten := Punkt(4, 3);
  r.Name := 'Feld';
  WriteLn(r.Name, ' ', Flaeche(r));
  with r, RechtsUnten do
  begin
    x := x * 2;
    Name := Name + 'chen'
  end;
  WriteLn(r.Name, ' ', Flaeche(r), ' ', SizeOf(TRechteck));
  f.Mitte := Punkt(5, 5);
  f.Art := Quadrat;
  f.Seite := 7;
  f.Gedreht := True;
  with f do
    case Art of
      Kreis: WriteLn('Kreis ', Radius);
      Quadrat: WriteLn('Quadrat ', Seite, ' ', Gedreht, ' ', Radius)
    end;
  poly.Anzahl := 4;
  for i := 1 to 4 do
    poly.Ecken[i] := Punkt(i, i * i);
  with poly do
    for i := 1 to Anzahl do
      with Ecken[i] do
        Write('(', x, ',', y, ')');
  WriteLn;
  WriteLn(Punkt(3, 4).y, ' ', poly.Ecken[3].y)
end.
