unit Protokoll;

interface

var
  Anzahl: Integer;

procedure Notiere(const s: string);

implementation

uses Werkzeug;

procedure Notiere(const s: string);
begin
  Inc(Anzahl);
  WriteLn('[', Anzahl, '] ', Rahmen(s))
end;

begin
  Anzahl := 0;
  WriteLn('Init Protokoll')
end.
