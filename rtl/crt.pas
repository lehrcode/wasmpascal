{ Unit Crt von WasmPascal.

  Bildschirmsteuerung ueber ANSI-Sequenzen, ohne Schatten-Bildschirm und
  ohne Cursor-Verfolgung: Die Ausgabe geht als Text an das Terminal, Umbruch
  und Rollen uebernimmt das Terminal (PLAN.md). Nicht enthalten sind GotoXY,
  WhereX, WhereY, Window und KeyPressed. Delay und ReadKey sind
  Anhaltepunkte des Schedulers. }
unit Crt;

interface

const
  { Farben (CGA) }
  Black = 0;
  Blue = 1;
  Green = 2;
  Cyan = 3;
  Red = 4;
  Magenta = 5;
  Brown = 6;
  LightGray = 7;
  DarkGray = 8;
  LightBlue = 9;
  LightGreen = 10;
  LightCyan = 11;
  LightRed = 12;
  LightMagenta = 13;
  Yellow = 14;
  White = 15;
  Blink = 128;

var
  TextAttr: Byte;      { Bits 0..3 Vordergrund, 4..6 Hintergrund, 7 Blinken }
  CheckBreak: Boolean; { ohne Wirkung, wie DirectVideo und CheckSnow }
  CheckEOF: Boolean;
  DirectVideo: Boolean;
  CheckSnow: Boolean;

procedure ClrScr;
procedure ClrEol;
procedure InsLine;
procedure DelLine;
procedure TextColor(Color: Byte);
procedure TextBackground(Color: Byte);
procedure HighVideo;
procedure LowVideo;
procedure NormVideo;
procedure Sound(Hz: Word);
procedure NoSound;
procedure Delay(MS: Word);
function ReadKey: Char;

implementation

const
  Esc = #27;
  NormAttr = LightGray;
  { CGA-Reihenfolge (Blau = 1, Rot = 4) auf ANSI-Reihenfolge (Rot = 1, Blau = 4) }
  AnsiFarbe: array[0..7] of Byte = (0, 4, 2, 6, 1, 5, 3, 7);

{ Schreibt TextAttr als ANSI-Attribute: 30..37 bzw. 90..97 fuer den
  Vordergrund (8..15 hell), 40..47 fuer den Hintergrund, 5 fuer Blinken. }
procedure SetzeAttribute;
var
  Vorne: Integer;
begin
  Vorne := TextAttr and 15;
  if Vorne >= 8 then
    Write(Esc, '[0;', 90 + AnsiFarbe[Vorne - 8])
  else
    Write(Esc, '[0;', 30 + AnsiFarbe[Vorne]);
  Write(';', 40 + AnsiFarbe[(TextAttr shr 4) and 7]);
  if TextAttr and Blink <> 0 then
    Write(';5');
  Write('m')
end;

procedure ClrScr;
begin
  Write(Esc, '[2J', Esc, '[H')
end;

procedure ClrEol;
begin
  Write(Esc, '[K')
end;

procedure InsLine;
begin
  Write(Esc, '[L')
end;

procedure DelLine;
begin
  Write(Esc, '[M')
end;

{ Farben 16..31 blinken wie in TP 3.0; Blink (128) ebenso. }
procedure TextColor(Color: Byte);
begin
  TextAttr := (TextAttr and $70) or (Color and 15);
  if (Color and $F0) <> 0 then
    TextAttr := TextAttr or Blink;
  SetzeAttribute
end;

procedure TextBackground(Color: Byte);
begin
  TextAttr := (TextAttr and $8F) or ((Color and 7) shl 4);
  SetzeAttribute
end;

procedure HighVideo;
begin
  TextAttr := TextAttr or 8;
  SetzeAttribute
end;

procedure LowVideo;
begin
  TextAttr := TextAttr and $F7;
  SetzeAttribute
end;

procedure NormVideo;
begin
  TextAttr := NormAttr;
  Write(Esc, '[0m')
end;

{ Wartet MS Millisekunden; die Ausgabe wird vorher geleert. }
procedure Delay(MS: Word);
begin
  Flush(Output);
  if _Suspend(2, MS) <> 0 then;
end;

{ Zeilenmodus: wartet, bis eine Zeile mit Enter abgeschlossen ist, liefert
  deren erstes Zeichen und verwirft den Rest samt Enter, damit ein folgendes
  ReadLn eine neue Zeile liest. Eine leere Zeile ergibt #13, das Dateiende
  #26. Sondertasten gibt es nicht. }
function ReadKey: Char;
var
  C: Char;
begin
  Read(C);
  if C = #10 then
    C := #13
  else if C <> #26 then
    ReadLn;
  ReadKey := C
end;

procedure Sound(Hz: Word);
begin
end;

procedure NoSound;
begin
end;

begin
  TextAttr := NormAttr;
  CheckBreak := True;
  CheckEOF := False;
  DirectVideo := True;
  CheckSnow := False
end.
