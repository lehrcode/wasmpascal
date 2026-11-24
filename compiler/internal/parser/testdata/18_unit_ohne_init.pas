unit Stapel;

interface

type
  TStapel = record
    Daten: array[1..100] of Integer;
    Oben: Integer
  end;

procedure Init(var s: TStapel);
procedure Push(var s: TStapel; x: Integer);
function Pop(var s: TStapel): Integer;
function Leer(const s: TStapel): Boolean;

implementation

procedure Init(var s: TStapel);
begin
  s.Oben := 0
end;

procedure Push(var s: TStapel; x: Integer);
begin
  Inc(s.Oben);
  s.Daten[s.Oben] := x
end;

function Pop;
begin
  Pop := s.Daten[s.Oben];
  Dec(s.Oben)
end;

function Leer(const s: TStapel): Boolean;
begin
  Leer := s.Oben = 0
end;

end.
